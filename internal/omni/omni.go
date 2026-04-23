package omni

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	resourcepb "github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	cosistate "github.com/cosi-project/runtime/pkg/state"
	omnispec "github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	omniapi "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	sysresources "github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/template/operations"
	"go.yaml.in/yaml/v4"
)

// ============================================================
// Client initialisation
// ============================================================

var (
	omniClient            *client.Client //nolint:unused
	omniState             cosistate.State
	omniEndpoint          string
	omniServiceAccountKey string
)

// clusterSnapshot caches the latest cluster IDs and tearing-down set from
// WatchClusters so reconcile can skip API round-trips.
var (
	clusterCacheMu    sync.RWMutex
	cachedClusterIDs  []string
	cachedTearingDown map[string]bool
	clusterCacheReady bool
)

// machineClassCache caches the latest MachineClass content from WatchMachineClasses.
var (
	mcCacheMu            sync.RWMutex
	cachedMachineClasses map[string]string // id -> YAML content
	mcCacheReady         bool
)

// CacheClusterSnapshot stores the latest IDs and tearing-down set from the
// watch so doReconcile can read them without an extra API call.
func CacheClusterSnapshot(allIDs []string, tearingDown map[string]bool) {
	clusterCacheMu.Lock()
	cachedClusterIDs = allIDs
	cachedTearingDown = tearingDown
	clusterCacheReady = true
	clusterCacheMu.Unlock()
}

// GetCachedClusterIDsWithPhases returns the watch-maintained cache.
// ok is false when the cache hasn't been populated yet (before first bootstrap).
func GetCachedClusterIDsWithPhases() (allIDs []string, tearingDown map[string]bool, ok bool) {
	clusterCacheMu.RLock()
	defer clusterCacheMu.RUnlock()
	if !clusterCacheReady {
		return nil, nil, false
	}
	ids := make([]string, len(cachedClusterIDs))
	copy(ids, cachedClusterIDs)
	td := make(map[string]bool, len(cachedTearingDown))
	for k, v := range cachedTearingDown {
		td[k] = v
	}
	return ids, td, true
}

// CacheMachineClassSnapshot stores the latest MachineClass content from the watch.
func CacheMachineClassSnapshot(content map[string]string) {
	mcCacheMu.Lock()
	cachedMachineClasses = content
	mcCacheReady = true
	mcCacheMu.Unlock()
}

// GetCachedMachineClasses returns the watch-maintained machine class cache.
// ok is false when the cache hasn't been populated yet.
func GetCachedMachineClasses() (map[string]string, bool) {
	mcCacheMu.RLock()
	defer mcCacheMu.RUnlock()
	if !mcCacheReady {
		return nil, false
	}
	out := make(map[string]string, len(cachedMachineClasses))
	for k, v := range cachedMachineClasses {
		out[k] = v
	}
	return out, true
}

// ClearCache resets the cluster and machine class caches. Must be called when
// switching to a different Omni instance so stale data from the previous
// instance does not pollute the new connection.
func ClearCache() {
	clusterCacheMu.Lock()
	cachedClusterIDs = nil
	cachedTearingDown = nil
	clusterCacheReady = false
	clusterCacheMu.Unlock()

	mcCacheMu.Lock()
	cachedMachineClasses = nil
	mcCacheReady = false
	mcCacheMu.Unlock()
}

// Init creates the Omni gRPC client. Must be called once at startup before
// any other function in this package.
func Init(endpoint, serviceAccountKey string) error {
	ClearCache()
	c, err := client.New(endpoint, client.WithServiceAccount(serviceAccountKey))
	if err != nil {
		return fmt.Errorf("failed to create Omni client: %w", err)
	}
	omniClient = c
	omniState = c.Omni().State()
	omniEndpoint = endpoint
	omniServiceAccountKey = serviceAccountKey
	return nil
}

// ============================================================
// Connectivity
// ============================================================

// Ping verifies Omni is reachable by opening a fresh connection each time,
// bypassing the cached omniState. Use this for health polling so that
// connectivity loss is detected even when the persistent gRPC client has
// stale cached state.
func Ping() error {
	return TestConnectivity(omniEndpoint, omniServiceAccountKey)
}

// CheckConnectivity verifies that the Omni API is reachable by reading the
// SysVersion resource.
func CheckConnectivity() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := safe.StateGet[*sysresources.SysVersion](ctx, omniState,
		sysresources.NewSysVersion(sysresources.SysVersionID).Metadata())
	return err
}

// TestConnectivity creates a throwaway client to verify credentials without
// touching the global omniClient state.
func TestConnectivity(endpoint, serviceAccountKey string) error {
	c, err := client.New(endpoint, client.WithServiceAccount(serviceAccountKey))
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer c.Close()
	st := c.Omni().State()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err = safe.StateGet[*sysresources.SysVersion](ctx, st,
		sysresources.NewSysVersion(sysresources.SysVersionID).Metadata())
	return err
}

// ============================================================
// Version
// ============================================================

// GetOmniVersion returns the Omni backend version string.
func GetOmniVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sv, err := safe.StateGet[*sysresources.SysVersion](ctx, omniState,
		sysresources.NewSysVersion(sysresources.SysVersionID).Metadata())
	if err != nil {
		return "unknown"
	}
	return sv.TypedSpec().Value.GetBackendVersion()
}

// ============================================================
// Machine Classes
// ============================================================

// Apply applies a YAML resource file to Omni (create or update any resource).
// Supports multi-document YAML files separated by ---.
func Apply(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return applyBytes(ctx, data, nil)
}

// ApplyIDs applies only the resources from file whose metadata ID is in allowedIDs.
// If allowedIDs is nil, all resources in the file are applied (same as Apply).
func ApplyIDs(file string, allowedIDs map[string]bool) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return applyBytes(ctx, data, allowedIDs)
}

func applyBytes(ctx context.Context, data []byte, allowedIDs map[string]bool) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var res resourcepb.YAMLResource
		if err := dec.Decode(&res); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("failed to decode resource: %w", err)
		}
		r := res.Resource()
		if allowedIDs != nil && !allowedIDs[r.Metadata().ID()] {
			continue
		}
		got, err := omniState.Get(ctx, r.Metadata())
		if cosistate.IsNotFoundError(err) {
			if createErr := omniState.Create(ctx, r); createErr != nil {
				return fmt.Errorf("failed to create %s %q: %w", r.Metadata().Type(), r.Metadata().ID(), createErr)
			}
		} else if err == nil {
			r.Metadata().SetVersion(got.Metadata().Version())
			if updateErr := omniState.Update(ctx, r); updateErr != nil {
				return fmt.Errorf("failed to update %s %q: %w", r.Metadata().Type(), r.Metadata().ID(), updateErr)
			}
		} else {
			return fmt.Errorf("failed to get %s %q: %w", r.Metadata().Type(), r.Metadata().ID(), err)
		}
	}
}

// MCDryRunResult holds the per-resource result of a dry run.
type MCDryRunResult struct {
	Diff string // empty = in sync
	Err  error  // non-nil = decode/API error for this resource
}

// MachineClassDryRunPerID runs a dry-run for every resource in file and returns
// a map of resource-ID → result. Callers can then handle each ID independently.
func MachineClassDryRunPerID(file string) (map[string]MCDryRunResult, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	results := map[string]MCDryRunResult{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var yamlRes resourcepb.YAMLResource
		if err := dec.Decode(&yamlRes); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to decode resource: %w", err)
		}
		fileRes := yamlRes.Resource()
		id := fileRes.Metadata().ID()
		liveRes, err := omniState.Get(ctx, fileRes.Metadata())
		if cosistate.IsNotFoundError(err) {
			fileYAML, specErr := marshalResourceYAML(fileRes)
			if specErr != nil {
				results[id] = MCDryRunResult{Diff: fmt.Sprintf("+ new: %s %q", fileRes.Metadata().Type(), id)}
			} else {
				results[id] = MCDryRunResult{Diff: fmt.Sprintf("+ new: %s %q\n+++ desired\n%s", fileRes.Metadata().Type(), id, fileYAML)}
			}
			continue
		}
		if err != nil {
			results[id] = MCDryRunResult{Err: err}
			continue
		}
		fileSpec, e1 := marshalSpecYAML(fileRes)
		liveSpec, e2 := marshalSpecYAML(liveRes)
		if e1 != nil || e2 != nil {
			results[id] = MCDryRunResult{Diff: fmt.Sprintf("~ changed: %s %q", fileRes.Metadata().Type(), id)}
			continue
		}
		if fileSpec != liveSpec {
			results[id] = MCDryRunResult{Diff: fmt.Sprintf("~ changed: %s %q\n--- live\n%s\n+++ desired\n%s",
				fileRes.Metadata().Type(), id, liveSpec, fileSpec)}
		} else {
			results[id] = MCDryRunResult{} // in sync
		}
	}
	return results, nil
}

// MachineClassDryRun checks whether a machine class file would change the live
// state. Returns a non-empty diff string if changes are needed, empty string if
// already in sync, and an error if the file is invalid.
func MachineClassDryRun(file string) (string, error) {
	perID, err := MachineClassDryRunPerID(file)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, r := range perID {
		if r.Err != nil {
			return "", r.Err
		}
		if r.Diff != "" {
			parts = append(parts, r.Diff)
		}
	}
	return strings.Join(parts, "\n"), nil
}

// marshalResourceYAML marshals a resource (metadata + spec) to a YAML string.
func marshalResourceYAML(r resource.Resource) (string, error) {
	v, err := resource.MarshalYAML(r)
	if err != nil {
		return "", err
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// marshalSpecYAML marshals only the spec portion of a resource to YAML.
func marshalSpecYAML(r resource.Resource) (string, error) {
	out, err := yaml.Marshal(r.Spec())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetMachineClassIDs returns all machine class IDs currently registered in Omni.
func GetMachineClassIDs() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	list, err := safe.StateListAll[*omniapi.MachineClass](ctx, omniState)
	if err != nil {
		return nil, err
	}
	var ids []string
	list.ForEach(func(r *omniapi.MachineClass) {
		ids = append(ids, r.Metadata().ID())
	})
	return ids, nil
}

// GetLiveMachineClass returns the live machine class state as a YAML string.
func GetLiveMachineClass(id string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mc, err := safe.StateGet[*omniapi.MachineClass](ctx, omniState, omniapi.NewMachineClass(id).Metadata())
	if err != nil {
		return "", err
	}
	return marshalResourceYAML(mc)
}

// GetAllLiveMachineClasses fetches all machine classes as YAML strings, keyed by ID.
func GetAllLiveMachineClasses() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	list, err := safe.StateListAll[*omniapi.MachineClass](ctx, omniState)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	var iterErr error
	list.ForEach(func(r *omniapi.MachineClass) {
		if iterErr != nil {
			return
		}
		y, e := marshalResourceYAML(r)
		if e != nil {
			iterErr = e
			return
		}
		result[r.Metadata().ID()] = y
	})
	return result, iterErr
}

// GetAllMachineHostnames returns a map of machine UUID -> hostname for every
// machine known to Omni. The hostname comes from the MachineStatus network spec.
func GetAllMachineHostnames() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	list, err := safe.StateListAll[*omniapi.MachineStatus](ctx, omniState)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	list.ForEach(func(r *omniapi.MachineStatus) {
		hn := r.TypedSpec().Value.GetNetwork().GetHostname()
		if hn != "" {
			result[r.Metadata().ID()] = hn
		}
	})
	return result, nil
}

// GetMachineSetNodes returns machine UUIDs grouped by MachineSet resource ID
// for a given cluster.  The map key is the MachineSet ID (e.g.
// "mycluster-control-planes") and the value is the list of machine UUIDs
// allocated to that MachineSet.
func GetMachineSetNodes(clusterID string) (map[string][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	list, err := safe.StateListAll[*omniapi.MachineSetNode](ctx, omniState,
		cosistate.WithLabelQuery(resource.LabelEqual(omniapi.LabelCluster, clusterID)))
	if err != nil {
		return nil, err
	}
	result := make(map[string][]string)
	list.ForEach(func(r *omniapi.MachineSetNode) {
		ms, _ := r.Metadata().Labels().Get(omniapi.LabelMachineSet)
		if ms != "" {
			result[ms] = append(result[ms], r.Metadata().ID())
		}
	})
	return result, nil
}

// DeleteMachineClass deletes a machine class from Omni by ID.
// Returns the error message as the first return value (for "still in use" checks).
func DeleteMachineClass(id string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	md := omniapi.NewMachineClass(id).Metadata()
	if err := omniState.TeardownAndDestroy(ctx, md); err != nil {
		return err.Error(), err
	}
	return "", nil
}

// ============================================================
// Cluster Templates
// ============================================================

// templateOpMu serialises all cluster-template operations that use os.Chdir,
// because os.Chdir is process-wide and not safe for concurrent use.
var templateOpMu sync.Mutex

// withTemplateDir chdirs to the directory of file, calls fn, then restores the
// original working directory. Must be called with templateOpMu held.
func withTemplateDir(file string, fn func() error) error {
	orig, err := os.Getwd()
	if err != nil {
		return err
	}
	dir := filepath.Dir(file)
	if err := os.Chdir(dir); err != nil {
		return err
	}
	defer os.Chdir(orig) //nolint:errcheck
	return fn()
}

// ClusterTemplateValidate validates a cluster template file (offline, no API access).
func ClusterTemplateValidate(file string) error {
	templateOpMu.Lock()
	defer templateOpMu.Unlock()

	return withTemplateDir(file, func() error {
		f, err := os.Open(filepath.Base(file))
		if err != nil {
			return err
		}
		defer f.Close() //nolint:errcheck
		return operations.ValidateTemplate(f)
	})
}

// ClusterTemplateSync syncs a cluster template to Omni (create/update/delete resources).
func ClusterTemplateSync(file string) error {
	templateOpMu.Lock()
	defer templateOpMu.Unlock()

	return withTemplateDir(file, func() error {
		f, err := os.Open(filepath.Base(file))
		if err != nil {
			return err
		}
		defer f.Close() //nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		return operations.SyncTemplate(ctx, f, io.Discard, omniState,
			operations.SyncOptions{})
	})
}

// ClusterTemplateDiff returns the human-readable diff for a cluster template.
// Returns empty string if there are no changes.
func ClusterTemplateDiff(file string) (string, error) {
	templateOpMu.Lock()
	defer templateOpMu.Unlock()

	var result string
	err := withTemplateDir(file, func() error {
		f, err := os.Open(filepath.Base(file))
		if err != nil {
			return err
		}
		defer f.Close() //nolint:errcheck
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var buf strings.Builder
		if err := operations.DiffTemplate(ctx, f, &buf, omniState); err != nil {
			return err
		}
		result = buf.String()
		return nil
	})
	return result, err
}

// ============================================================
// Clusters
// ============================================================

// GetClusterIDs returns all cluster IDs currently registered in Omni.
func GetClusterIDs() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	list, err := safe.StateListAll[*omniapi.Cluster](ctx, omniState)
	if err != nil {
		return nil, err
	}
	var ids []string
	list.ForEach(func(r *omniapi.Cluster) {
		ids = append(ids, r.Metadata().ID())
	})
	return ids, nil
}

// GetClusterIDsWithPhases returns all cluster IDs and a set of IDs that are
// currently in the TearingDown phase (i.e. being deleted in Omni).
func GetClusterIDsWithPhases() (allIDs []string, tearingDown map[string]bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	list, err := safe.StateListAll[*omniapi.Cluster](ctx, omniState)
	if err != nil {
		return nil, nil, err
	}
	tearingDown = make(map[string]bool)
	list.ForEach(func(r *omniapi.Cluster) {
		id := r.Metadata().ID()
		allIDs = append(allIDs, id)
		if r.Metadata().Phase() == resource.PhaseTearingDown {
			tearingDown[id] = true
		}
	})
	return allIDs, tearingDown, nil
}

// WatchClusterConfigChanges watches MachineSet resources for changes made
// directly in Omni (outside of omni-cd). Calls onChanged with the affected
// cluster ID on every Created, Updated, or Destroyed event.
// Cluster-level config changes are delivered via the onClusterChange callback
// in WatchClusters, so no separate Cluster watch is needed here.
// Does NOT bootstrap initial state — only fires on actual changes.
// Blocks until ctx is cancelled. Run in a goroutine.
func WatchClusterConfigChanges(ctx context.Context, onChanged func(clusterID string)) error {
	machineSetCh := make(chan safe.WrappedStateEvent[*omniapi.MachineSet], 64)
	if err := safe.StateWatchKind[*omniapi.MachineSet](ctx, omniState, omniapi.NewMachineSet("").Metadata(), machineSetCh); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-machineSetCh:
			switch event.Type() {
			case cosistate.Created, cosistate.Updated, cosistate.Destroyed:
				if r, err := event.Resource(); err == nil {
					if clusterID, ok := r.Metadata().Labels().Get(omniapi.LabelCluster); ok && clusterID != "" {
						onChanged(clusterID)
					}
				}
			}
		}
	}
}

// WatchMachineSetNodes watches MachineSetNode resources and calls onChanged with
// the cluster ID whenever a node is created, updated, or destroyed. This fires
// when Omni assigns or removes a machine from a MachineSet — e.g. during cluster
// creation or pool scaling — enabling the UI to reflect machine topology changes
// in real time without waiting for the next full reconcile cycle.
// Does NOT bootstrap initial state — only fires on actual changes.
// Blocks until ctx is cancelled. Run in a goroutine.
func WatchMachineSetNodes(ctx context.Context, onChanged func(clusterID string)) error {
	ch := make(chan safe.WrappedStateEvent[*omniapi.MachineSetNode], 64)
	msnMeta := resource.NewMetadata("default", omniapi.MachineSetNodeType, "", resource.VersionUndefined)
	if err := safe.StateWatchKind[*omniapi.MachineSetNode](ctx, omniState, msnMeta, ch); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-ch:
			switch event.Type() {
			case cosistate.Created, cosistate.Updated, cosistate.Destroyed:
				if r, err := event.Resource(); err == nil {
					if clusterID, ok := r.Metadata().Labels().Get(omniapi.LabelCluster); ok && clusterID != "" {
						onChanged(clusterID)
					}
				}
			}
		}
	}
}

// WatchMachineClasses watches MachineClass resources and maintains an in-memory
// cache of id -> YAML content. onUpdate is called after bootstrap and on every change.
// Blocks until ctx is cancelled. Run in a goroutine.
func WatchMachineClasses(ctx context.Context, onUpdate func(content map[string]string)) error {
	ch := make(chan safe.WrappedStateEvent[*omniapi.MachineClass], 64)
	if err := safe.StateWatchKind[*omniapi.MachineClass](ctx, omniState, omniapi.NewMachineClass("").Metadata(), ch,
		cosistate.WithBootstrapContents(true)); err != nil {
		return err
	}

	cache := make(map[string]string)

	snapshot := func() map[string]string {
		out := make(map[string]string, len(cache))
		for k, v := range cache {
			out[k] = v
		}
		return out
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-ch:
			switch event.Type() {
			case cosistate.Bootstrapped:
				onUpdate(snapshot())
			case cosistate.Created, cosistate.Updated:
				r, err := event.Resource()
				if err != nil {
					continue
				}
				y, err := marshalResourceYAML(r)
				if err != nil {
					continue
				}
				cache[r.Metadata().ID()] = y
				onUpdate(snapshot())
			case cosistate.Destroyed:
				r, err := event.Resource()
				if err == nil {
					delete(cache, r.Metadata().ID())
					onUpdate(snapshot())
				}
			}
		}
	}
}

// WatchClusters watches ClusterStatus, ControlPlaneStatus, and Cluster resources
// in real time. It calls onUpdate with the full current snapshot whenever anything
// changes. onClusterChange (may be nil) is called on every Cluster Updated event
// so callers can detect config changes without a separate watch.
// Blocks until ctx is cancelled — run in a goroutine.
// Returns an error only if the initial watch setup fails.
func WatchClusters(
	ctx context.Context,
	onUpdate func(statuses map[string]ClusterStatus, allIDs []string, tearingDown map[string]bool),
	onClusterChange func(clusterID string),
) error {
	statusCh := make(chan safe.WrappedStateEvent[*omniapi.ClusterStatus], 64)
	if err := safe.StateWatchKind[*omniapi.ClusterStatus](ctx, omniState, omniapi.NewClusterStatus("").Metadata(), statusCh,
		cosistate.WithBootstrapContents(true)); err != nil {
		return err
	}

	clusterCh := make(chan safe.WrappedStateEvent[*omniapi.Cluster], 64)
	if err := safe.StateWatchKind[*omniapi.Cluster](ctx, omniState, omniapi.NewCluster("").Metadata(), clusterCh,
		cosistate.WithBootstrapContents(true)); err != nil {
		return err
	}

	cpCh := make(chan safe.WrappedStateEvent[*omniapi.ControlPlaneStatus], 64)
	if err := safe.StateWatchKind[*omniapi.ControlPlaneStatus](ctx, omniState, omniapi.NewControlPlaneStatus("").Metadata(), cpCh,
		cosistate.WithBootstrapContents(true)); err != nil {
		return err
	}

	msStatusCh := make(chan safe.WrappedStateEvent[*omniapi.MachineSetStatus], 64)
	if err := safe.StateWatchKind[*omniapi.MachineSetStatus](ctx, omniState, omniapi.NewMachineSetStatus("").Metadata(), msStatusCh,
		cosistate.WithBootstrapContents(true)); err != nil {
		return err
	}

	statusCache := make(map[string]ClusterStatus)
	clusterPhases := make(map[string]resource.Phase)
	// msPhasesByCluster: clusterID -> msID -> MachineSetPhase
	// Used to derive the cluster-level operational phase from individual MachineSet phases.
	msPhasesByCluster := make(map[string]map[string]omnispec.MachineSetPhase)
	statusReady := false
	clusterReady := false

	// derivedClusterPhase returns the operational phase string for a cluster.
	// When the cluster itself is tearing down, MachineSetPhase_Destroying is the
	// expected state and maps to "destroying". When the cluster is alive,
	// MachineSetPhase_Destroying means a worker group is being removed (scale-to-zero)
	// and is treated as "scaling-down" to avoid confusing the user.
	derivedClusterPhase := func(clusterID string, clusterTearingDown bool) string {
		phases, ok := msPhasesByCluster[clusterID]
		if !ok || len(phases) == 0 {
			return "running"
		}
		best := omnispec.MachineSetPhase_Running
		for _, p := range phases {
			switch {
			case p == omnispec.MachineSetPhase_Destroying:
				if clusterTearingDown {
					return "destroying"
				}
				// Worker group being removed — treat as scaling-down.
				if best != omnispec.MachineSetPhase_ScalingDown {
					best = omnispec.MachineSetPhase_ScalingDown
				}
			case p == omnispec.MachineSetPhase_ScalingDown && best != omnispec.MachineSetPhase_ScalingDown:
				best = p
			case p == omnispec.MachineSetPhase_ScalingUp && best != omnispec.MachineSetPhase_ScalingDown:
				best = p
			case p == omnispec.MachineSetPhase_Reconfiguring && best == omnispec.MachineSetPhase_Running:
				best = p
			}
		}
		switch best {
		case omnispec.MachineSetPhase_ScalingDown:
			return "scaling-down"
		case omnispec.MachineSetPhase_ScalingUp:
			return "scaling-up"
		case omnispec.MachineSetPhase_Reconfiguring:
			return "reconfiguring"
		default:
			return "running"
		}
	}

	notify := func() {
		if !statusReady || !clusterReady {
			return
		}
		allIDs := make([]string, 0, len(clusterPhases))
		td := make(map[string]bool)
		for id, phase := range clusterPhases {
			allIDs = append(allIDs, id)
			if phase == resource.PhaseTearingDown {
				td[id] = true
			}
		}
		statuses := make(map[string]ClusterStatus, len(statusCache))
		for k, v := range statusCache {
			v.Phase = derivedClusterPhase(k, td[k])
			statuses[k] = v
		}
		onUpdate(statuses, allIDs, td)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-statusCh:
			switch event.Type() {
			case cosistate.Bootstrapped:
				statusReady = true
			case cosistate.Created, cosistate.Updated:
				r, err := event.Resource()
				if err != nil {
					continue
				}
				spec := r.TypedSpec().Value
				// Preserve existing ControlPlane fields when updating ClusterStatus.
				existing := statusCache[r.Metadata().ID()]
				cs := ClusterStatus{
					Ready:              spec.GetReady(),
					KubernetesAPIReady: spec.GetKubernetesAPIReady(),
					ControlPlaneReady:  spec.GetControlplaneReady(),
					EtcdStatus:         existing.EtcdStatus,
					WireGuardStatus:    existing.WireGuardStatus,
					BackupEnabled:      existing.BackupEnabled,
					LastBackupTime:     existing.LastBackupTime,
					Phase:              clusterPhaseString(spec.GetPhase()),
				}
				if cs.EtcdStatus == "" {
					cs.EtcdStatus = "unknown"
				}
				if cs.WireGuardStatus == "" {
					cs.WireGuardStatus = "unknown"
				}
				if m := spec.GetMachines(); m != nil {
					cs.MachinesHealthy = int(m.GetHealthy())
					cs.MachinesTotal = int(m.GetTotal())
				}
				statusCache[r.Metadata().ID()] = cs
			case cosistate.Destroyed:
				r, err := event.Resource()
				if err == nil {
					delete(statusCache, r.Metadata().ID())
				}
			}
			notify()
		case event := <-cpCh:
			switch event.Type() {
			case cosistate.Created, cosistate.Updated:
				r, err := event.Resource()
				if err != nil {
					continue
				}
				clusterID, ok := r.Metadata().Labels().Get(omniapi.LabelCluster)
				if !ok || clusterID == "" {
					continue
				}
				cs := statusCache[clusterID]
				for _, cond := range r.TypedSpec().Value.GetConditions() {
					ready := cond.GetStatus() == omnispec.ControlPlaneStatusSpec_Condition_Ready
					switch cond.GetType() {
					case omnispec.ConditionType_Etcd:
						if ready {
							cs.EtcdStatus = "ok"
						} else {
							cs.EtcdStatus = "not-ready"
						}
					case omnispec.ConditionType_WireguardConnection:
						if ready {
							cs.WireGuardStatus = "ok"
						} else {
							cs.WireGuardStatus = "not-ready"
						}
					}
				}
				statusCache[clusterID] = cs
			case cosistate.Destroyed:
				r, err := event.Resource()
				if err == nil {
					clusterID, ok := r.Metadata().Labels().Get(omniapi.LabelCluster)
					if ok && clusterID != "" {
						cs := statusCache[clusterID]
						cs.EtcdStatus = "unknown"
						cs.WireGuardStatus = "unknown"
						statusCache[clusterID] = cs
					}
				}
			}
			notify()
		case event := <-clusterCh:
			switch event.Type() {
			case cosistate.Bootstrapped:
				clusterReady = true
			case cosistate.Created, cosistate.Updated:
				r, err := event.Resource()
				if err != nil {
					continue
				}
				id := r.Metadata().ID()
				clusterPhases[id] = r.Metadata().Phase()
				if onClusterChange != nil {
					onClusterChange(id)
				}
			case cosistate.Destroyed:
				r, err := event.Resource()
				if err == nil {
					id := r.Metadata().ID()
					delete(clusterPhases, id)
					delete(msPhasesByCluster, id)
					if onClusterChange != nil {
						onClusterChange(id)
					}
				}
			}
			notify()
		case event := <-msStatusCh:
			switch event.Type() {
			case cosistate.Created, cosistate.Updated:
				r, err := event.Resource()
				if err != nil {
					continue
				}
				clusterID, ok := r.Metadata().Labels().Get(omniapi.LabelCluster)
				if !ok || clusterID == "" {
					continue
				}
				if msPhasesByCluster[clusterID] == nil {
					msPhasesByCluster[clusterID] = make(map[string]omnispec.MachineSetPhase)
				}
				msPhasesByCluster[clusterID][r.Metadata().ID()] = r.TypedSpec().Value.GetPhase()
			case cosistate.Destroyed:
				r, err := event.Resource()
				if err == nil {
					clusterID, ok := r.Metadata().Labels().Get(omniapi.LabelCluster)
					if ok && clusterID != "" {
						delete(msPhasesByCluster[clusterID], r.Metadata().ID())
					}
				}
			}
			notify()
		}
	}
}

// ClusterStatus holds relevant status fields from the Omni ClusterStatus resource.
type ClusterStatus struct {
	Ready              bool
	KubernetesAPIReady bool
	ControlPlaneReady  bool
	MachinesHealthy    int
	MachinesTotal      int
	EtcdStatus         string // "ok", "not-ready", "unknown"
	WireGuardStatus    string // "ok", "not-ready", "unknown"
	LastBackupTime     time.Time
	BackupEnabled      bool
	Phase              string // "running", "scaling-up", "scaling-down", "destroying", "unknown"
}

// clusterPhaseString converts an Omni ClusterStatusSpec_Phase to a short string.
func clusterPhaseString(p omnispec.ClusterStatusSpec_Phase) string {
	switch p {
	case omnispec.ClusterStatusSpec_SCALING_UP:
		return "scaling-up"
	case omnispec.ClusterStatusSpec_SCALING_DOWN:
		return "scaling-down"
	case omnispec.ClusterStatusSpec_DESTROYING:
		return "destroying"
	case omnispec.ClusterStatusSpec_RUNNING:
		return "running"
	default:
		return "unknown"
	}
}

// GetAllClusterReadyStatuses fetches ready-status fields for every cluster.
// Returns a map of cluster ID -> ClusterStatus.
func GetAllClusterReadyStatuses() (map[string]ClusterStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// --- ClusterStatus: ready + machine counts ---
	list, err := safe.StateListAll[*omniapi.ClusterStatus](ctx, omniState)
	if err != nil {
		return nil, err
	}
	result := make(map[string]ClusterStatus)
	list.ForEach(func(r *omniapi.ClusterStatus) {
		spec := r.TypedSpec().Value
		cs := ClusterStatus{
			Ready:              spec.GetReady(),
			KubernetesAPIReady: spec.GetKubernetesAPIReady(),
			ControlPlaneReady:  spec.GetControlplaneReady(),
			EtcdStatus:         "unknown",
			WireGuardStatus:    "unknown",
			Phase:              clusterPhaseString(spec.GetPhase()),
		}
		if m := spec.GetMachines(); m != nil {
			cs.MachinesHealthy = int(m.GetHealthy())
			cs.MachinesTotal = int(m.GetTotal())
		}
		result[r.Metadata().ID()] = cs
	})

	// --- ControlPlaneStatus: etcd + wireguard conditions ---
	cpList, err := safe.StateListAll[*omniapi.ControlPlaneStatus](ctx, omniState)
	if err == nil {
		cpList.ForEach(func(r *omniapi.ControlPlaneStatus) {
			clusterID, ok := r.Metadata().Labels().Get(omniapi.LabelCluster)
			if !ok || clusterID == "" {
				return
			}
			cs, ok := result[clusterID]
			if !ok {
				return
			}
			for _, cond := range r.TypedSpec().Value.GetConditions() {
				ready := cond.GetStatus() == omnispec.ControlPlaneStatusSpec_Condition_Ready
				switch cond.GetType() {
				case omnispec.ConditionType_Etcd:
					if ready {
						cs.EtcdStatus = "ok"
					} else {
						cs.EtcdStatus = "not-ready"
					}
				case omnispec.ConditionType_WireguardConnection:
					if ready {
						cs.WireGuardStatus = "ok"
					} else {
						cs.WireGuardStatus = "not-ready"
					}
				}
			}
			result[clusterID] = cs
		})
	}

	// --- EtcdBackupOverallStatus: global last backup time ---
	backupStatus, err := safe.StateGet[*omniapi.EtcdBackupOverallStatus](ctx, omniState,
		omniapi.NewEtcdBackupOverallStatus().Metadata())
	if err == nil {
		spec := backupStatus.TypedSpec().Value
		backupEnabled := spec.GetConfigurationName() != "disabled"
		var lastBackup time.Time
		if backupEnabled {
			if lbs := spec.GetLastBackupStatus(); lbs != nil {
				if t := lbs.GetLastBackupTime(); t != nil {
					lastBackup = t.AsTime()
				}
			}
		}
		for id, cs := range result {
			cs.BackupEnabled = backupEnabled
			cs.LastBackupTime = lastBackup
			result[id] = cs
		}
	}

	return result, nil
}

// DeleteCluster deletes a cluster from Omni.
func DeleteCluster(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return operations.DeleteCluster(ctx, id, io.Discard, omniState,
		operations.SyncOptions{})
}

// ExportCluster exports a cluster configuration as cluster template YAML.
func ExportCluster(id string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var buf strings.Builder
	_, err := operations.ExportTemplate(ctx, omniState, id, false, &buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// GetLiveCluster returns the live cluster state (same as ExportCluster).
func GetLiveCluster(id string) (string, error) {
	return ExportCluster(id)
}

// GetAllLiveClusters fetches all cluster templates in parallel.
// Returns a map of cluster name -> YAML content.
func GetAllLiveClusters() (map[string]string, error) {
	ids, err := GetClusterIDs()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return make(map[string]string), nil
	}

	type result struct {
		id      string
		content string
	}
	resultChan := make(chan result, len(ids))
	for _, id := range ids {
		go func(clusterID string) {
			content, _ := ExportCluster(clusterID)
			resultChan <- result{id: clusterID, content: content}
		}(id)
	}
	resultMap := make(map[string]string)
	for range ids {
		r := <-resultChan
		if r.content != "" {
			resultMap[r.id] = r.content
		}
	}
	return resultMap, nil
}

// GetMachineClassCreatedAt returns the creation timestamp from the MachineClass resource metadata.
func GetMachineClassCreatedAt(id string) time.Time {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mc, err := safe.StateGet[*omniapi.MachineClass](ctx, omniState, omniapi.NewMachineClass(id).Metadata())
	if err != nil {
		return time.Time{}
	}
	return mc.Metadata().Created()
}

// GetClusterCreatedAt returns the creation timestamp from the Cluster resource metadata.
func GetClusterCreatedAt(id string) time.Time {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cluster, err := safe.StateGet[*omniapi.Cluster](ctx, omniState, omniapi.NewCluster(id).Metadata())
	if err != nil {
		return time.Time{}
	}
	return cluster.Metadata().Created()
}

// IsClusterTearingDown returns true if the cluster resource in Omni is currently
// in the tearingdown phase (i.e. a delete is in progress on Omni's side).
// Returns false if the cluster no longer exists or an error occurs.
func IsClusterTearingDown(id string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cluster, err := safe.StateGet[*omniapi.Cluster](ctx, omniState, omniapi.NewCluster(id).Metadata())
	if err != nil {
		// Cluster not found or other error — treat as already gone, not still deleting.
		return false
	}
	return cluster.Metadata().Phase() == resource.PhaseTearingDown
}

// IsClusterTemplateManaged checks if a cluster has the managed-by-cluster-templates annotation.
func IsClusterTemplateManaged(id string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cluster, err := safe.StateGet[*omniapi.Cluster](ctx, omniState, omniapi.NewCluster(id).Metadata())
	if err != nil {
		return false
	}
	_, ok := cluster.Metadata().Annotations().Get(omniapi.ResourceManagedByClusterTemplates)
	return ok
}

// ============================================================
// Cluster Template Parsing
// ============================================================

// WorkerGroup holds information about a single Workers pool from a cluster template.
type WorkerGroup struct {
	Name         string
	Count        int
	MachineClass string
	Machines     []string
	Extensions   []string
}

// ClusterTemplateInfo holds parsed information from an exported cluster template.
type ClusterTemplateInfo struct {
	TalosVersion             string
	KubernetesVersion        string
	ControlPlaneName         string
	ControlPlaneCount        int
	ControlPlaneMachineClass string
	ControlPlaneMachines     []string
	ControlPlaneExtensions   []string
	WorkerGroups             []WorkerGroup
	// Extensions defined at the cluster level (kind: Cluster => systemExtensions)
	ClusterExtensions []string
	// Extensions defined per individual machine (kind: Machine => systemExtensions, keyed by machine name/UUID)
	MachineExtensions map[string][]string
}

// ParseClusterTemplate parses a cluster template YAML and extracts version and
// node group information. It supports two formats:
//   - The legacy `kind: ControlPlane / Workers` template format
//   - The Omni resource format (`type: MachineSets.omni.sidero.dev`) produced
//     by `omnictl cluster template render`
func ParseClusterTemplate(yamlContent string) ClusterTemplateInfo {
	// Detect format: resource format uses "type: MachineSets.omni.sidero.dev"
	for _, line := range strings.Split(yamlContent, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "type: MachineSets.omni.sidero.dev") ||
			strings.HasPrefix(t, "type: Clusters.omni.sidero.dev") ||
			strings.HasPrefix(t, "type: ExtensionsConfigurations.omni.sidero.dev") {
			return parseResourceFormat(yamlContent)
		}
	}
	return parseLegacyFormat(yamlContent)
}

// parseLegacyFormat handles the `kind: Cluster / ControlPlane / Workers` template format.
func parseLegacyFormat(yamlContent string) ClusterTemplateInfo {
	var info ClusterTemplateInfo

	// Scan full content for version fields.
	var inTalos, inKubernetes bool
	for _, line := range strings.Split(yamlContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if info.TalosVersion == "" && strings.HasPrefix(trimmed, "talosVersion:") {
			info.TalosVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "talosVersion:"))
		}
		if info.KubernetesVersion == "" && strings.HasPrefix(trimmed, "kubernetesVersion:") {
			info.KubernetesVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "kubernetesVersion:"))
		}
		if trimmed == "talos:" {
			inTalos = true
			inKubernetes = false
			continue
		}
		if trimmed == "kubernetes:" {
			inKubernetes = true
			inTalos = false
			continue
		}
		if strings.HasPrefix(trimmed, "version:") {
			v := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
			if inTalos && info.TalosVersion == "" {
				info.TalosVersion = v
			}
			if inKubernetes && info.KubernetesVersion == "" {
				info.KubernetesVersion = v
			}
			inTalos = false
			inKubernetes = false
			continue
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			inTalos = false
			inKubernetes = false
		}
	}

	machineExtensions := make(map[string][]string)

	for _, doc := range strings.Split(yamlContent, "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var kind, docName string
		var inMachines, inMachineClass, inSystemExtensions bool
		var machineClassSize int
		var machineClassName string
		var machines, systemExtensions []string

		for _, line := range strings.Split(doc, "\n") {
			trimmed := strings.TrimSpace(line)
			// Only treat `kind:` and `name:` as document-level declarations when they
			// are at the root level (no leading whitespace). Indented `kind:` fields
			// inside inline patches (e.g. `kind: UserVolumeConfig`) must not overwrite
			// the document kind — doing so drops the entire worker group from parsing.
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && strings.HasPrefix(trimmed, "kind:") {
				kind = strings.TrimSpace(strings.TrimPrefix(trimmed, "kind:"))
				inMachines, inMachineClass, inSystemExtensions = false, false, false
				continue
			}
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && strings.HasPrefix(trimmed, "name:") {
				docName = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				continue
			}
			switch kind {
			case "ControlPlane", "Workers":
				// A new unindented key resets all section flags.
				if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(trimmed, "- ") {
					inMachines, inMachineClass, inSystemExtensions = false, false, false
				}
				if strings.HasPrefix(trimmed, "machines:") {
					inMachines = true
					inMachineClass, inSystemExtensions = false, false
				} else if strings.HasPrefix(trimmed, "machineClass:") {
					inMachineClass = true
					inMachines, inSystemExtensions = false, false
				} else if strings.HasPrefix(trimmed, "systemExtensions:") {
					inSystemExtensions = true
					inMachines, inMachineClass = false, false
				} else if inMachines && strings.HasPrefix(trimmed, "- ") {
					machines = append(machines, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				} else if inSystemExtensions && strings.HasPrefix(trimmed, "- ") {
					systemExtensions = append(systemExtensions, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				} else if inMachineClass && strings.HasPrefix(trimmed, "name:") {
					machineClassName = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				} else if strings.HasPrefix(trimmed, "size:") {
					parts := strings.Fields(trimmed)
					if len(parts) >= 2 {
						machineClassSize, _ = strconv.Atoi(parts[1])
					}
				}
			case "Cluster", "Machine":
				if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(trimmed, "- ") {
					if strings.HasPrefix(trimmed, "systemExtensions:") {
						inSystemExtensions = true
					} else {
						inSystemExtensions = false
					}
				} else if inSystemExtensions && strings.HasPrefix(trimmed, "- ") {
					systemExtensions = append(systemExtensions, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				}
			case "Patch":
				// Config patches are not rendered in the topology graph — skip.
			}
		}

		switch kind {
		case "ControlPlane":
			if info.ControlPlaneName == "" {
				info.ControlPlaneName = "control-planes"
			}
			if machineClassName != "" {
				info.ControlPlaneMachineClass = machineClassName
				info.ControlPlaneCount = machineClassSize
			} else {
				info.ControlPlaneMachines = machines
				info.ControlPlaneCount = len(machines)
			}
			info.ControlPlaneExtensions = systemExtensions
		case "Workers":
			wg := WorkerGroup{Name: docName}
			if machineClassName != "" {
				wg.MachineClass = machineClassName
				wg.Count = machineClassSize
			} else {
				wg.Machines = machines
				wg.Count = len(machines)
			}
			wg.Extensions = systemExtensions
			info.WorkerGroups = append(info.WorkerGroups, wg)
		case "Cluster":
			if len(systemExtensions) > 0 {
				info.ClusterExtensions = append(info.ClusterExtensions, systemExtensions...)
			}
		case "Machine":
			if docName != "" && len(systemExtensions) > 0 {
				machineExtensions[docName] = append(machineExtensions[docName], systemExtensions...)
			}
		}
	}

	if len(machineExtensions) > 0 {
		info.MachineExtensions = machineExtensions
	}

	return info
}

// parseResourceFormat handles the Omni resource format produced by
// `omnictl cluster template render`, which uses typed documents with
// metadata labels to express roles and relationships.
func parseResourceFormat(yamlContent string) ClusterTemplateInfo {
	var info ClusterTemplateInfo

	type msEntry struct {
		id           string
		clusterID    string
		isCP         bool
		isWorker     bool
		machineClass string
		count        int
	}
	type extEntry struct {
		machineSetID string
		extensions   []string
	}
	type machExtEntry struct {
		machineID  string
		extensions []string
	}

	var msEntries []msEntry
	var extEntries []extEntry
	var machExtEntries []machExtEntry

	for _, doc := range strings.Split(yamlContent, "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var docType, docID, clusterID, machineSetID, clusterMachineID string
		var isCP, isWorker bool
		var machineClass string
		var machineCount int
		var extensions []string
		var section, subSection string

		for _, line := range strings.Split(doc, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			trimmed := strings.TrimSpace(line)

			if indent == 0 {
				switch trimmed {
				case "metadata:":
					section = "metadata"
				case "spec:":
					section = "spec"
				default:
					section = ""
				}
				subSection = ""
				continue
			}

			if indent == 2 {
				switch section {
				case "metadata":
					switch {
					case trimmed == "labels:":
						subSection = "labels"
					case strings.HasPrefix(trimmed, "type:"):
						docType = strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
						subSection = ""
					case strings.HasPrefix(trimmed, "id:"):
						docID = strings.TrimSpace(strings.TrimPrefix(trimmed, "id:"))
						subSection = ""
					default:
						subSection = ""
					}
				case "spec":
					switch {
					case strings.HasPrefix(trimmed, "machineallocation:"):
						subSection = "machineallocation"
					case strings.HasPrefix(trimmed, "extensions:"):
						subSection = "extensions"
					case strings.HasPrefix(strings.ToLower(trimmed), "talosversion:"):
						if info.TalosVersion == "" {
							info.TalosVersion = strings.TrimSpace(trimmed[strings.Index(trimmed, ":")+1:])
						}
						subSection = ""
					case strings.HasPrefix(strings.ToLower(trimmed), "kubernetesversion:"):
						if info.KubernetesVersion == "" {
							info.KubernetesVersion = strings.TrimSpace(trimmed[strings.Index(trimmed, ":")+1:])
						}
						subSection = ""
					default:
						subSection = ""
					}
				}
				continue
			}

			if indent == 4 {
				switch subSection {
				case "labels":
					switch {
					case strings.HasPrefix(trimmed, "omni.sidero.dev/cluster:"):
						clusterID = strings.TrimSpace(strings.TrimPrefix(trimmed, "omni.sidero.dev/cluster:"))
					case strings.HasPrefix(trimmed, "omni.sidero.dev/role-controlplane"):
						isCP = true
					case strings.HasPrefix(trimmed, "omni.sidero.dev/role-worker"):
						isWorker = true
					case strings.HasPrefix(trimmed, "omni.sidero.dev/machine-set:"):
						machineSetID = strings.TrimSpace(strings.TrimPrefix(trimmed, "omni.sidero.dev/machine-set:"))
					case strings.HasPrefix(trimmed, "omni.sidero.dev/cluster-machine:"):
						clusterMachineID = strings.TrimSpace(strings.TrimPrefix(trimmed, "omni.sidero.dev/cluster-machine:"))
					}
				case "machineallocation":
					switch {
					case strings.HasPrefix(trimmed, "name:"):
						machineClass = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
					case strings.HasPrefix(trimmed, "machinecount:"):
						parts := strings.Fields(trimmed)
						if len(parts) >= 2 {
							machineCount, _ = strconv.Atoi(parts[1])
						}
					}
				case "extensions":
					if strings.HasPrefix(trimmed, "- ") {
						extensions = append(extensions, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
					}
				}
			}
		}

		switch {
		case strings.Contains(docType, "MachineSets"):
			if docID == "" {
				continue
			}
			msEntries = append(msEntries, msEntry{
				id: docID, clusterID: clusterID,
				isCP: isCP, isWorker: isWorker,
				machineClass: machineClass, count: machineCount,
			})
		case strings.Contains(docType, "ExtensionsConfigurations"):
			if machineSetID != "" {
				extEntries = append(extEntries, extEntry{
					machineSetID: machineSetID,
					extensions:   extensions,
				})
			} else if clusterMachineID != "" {
				machExtEntries = append(machExtEntries, machExtEntry{
					machineID:  clusterMachineID,
					extensions: extensions,
				})
			}
		}
	}

	// Build extension lookup by machine-set ID
	extMap := make(map[string][]string)
	for _, e := range extEntries {
		extMap[e.machineSetID] = e.extensions
	}

	for _, ms := range msEntries {
		exts := extMap[ms.id]
		// Strip cluster-id prefix from the display name (e.g. "cluster-6-workers" => "workers")
		displayName := ms.id
		if ms.clusterID != "" && strings.HasPrefix(displayName, ms.clusterID+"-") {
			displayName = displayName[len(ms.clusterID)+1:]
		}
		switch {
		case ms.isCP:
			info.ControlPlaneName = displayName
			info.ControlPlaneCount = ms.count
			info.ControlPlaneMachineClass = ms.machineClass
			info.ControlPlaneExtensions = exts
		case ms.isWorker:
			info.WorkerGroups = append(info.WorkerGroups, WorkerGroup{
				Name:         displayName,
				Count:        ms.count,
				MachineClass: ms.machineClass,
				Extensions:   exts,
			})
		}
	}

	if len(machExtEntries) > 0 {
		info.MachineExtensions = make(map[string][]string)
		for _, e := range machExtEntries {
			info.MachineExtensions[e.machineID] = e.extensions
		}
	}
	return info
}
