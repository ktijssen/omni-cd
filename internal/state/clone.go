package state

// cloneResources returns a deep copy of a ResourceInfo slice safe to traverse
// concurrently with mutations to the source slice. Strings are immutable so
// they're safe to share; only nested slices and maps need copying.
func cloneResources(in []ResourceInfo) []ResourceInfo {
	if in == nil {
		return nil
	}
	out := make([]ResourceInfo, len(in))
	for i, r := range in {
		out[i] = r
		out[i].ControlPlane = cloneNodeGroup(r.ControlPlane)
		if r.Workers != nil {
			workers := make([]NodeGroup, len(r.Workers))
			for j, wg := range r.Workers {
				workers[j] = cloneNodeGroup(wg)
			}
			out[i].Workers = workers
		}
		if r.ClusterExtensions != nil {
			out[i].ClusterExtensions = append([]string(nil), r.ClusterExtensions...)
		}
		out[i].MachineExtensions = cloneStringSliceMap(r.MachineExtensions)
		out[i].MachineHostnames = cloneStringMap(r.MachineHostnames)
	}
	return out
}

func cloneNodeGroup(in NodeGroup) NodeGroup {
	out := in
	if in.Machines != nil {
		out.Machines = append([]string(nil), in.Machines...)
	}
	if in.Extensions != nil {
		out.Extensions = append([]string(nil), in.Extensions...)
	}
	return out
}

func cloneStringSliceMap(m map[string][]string) map[string][]string {
	if m == nil {
		return nil
	}
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneStringBoolMap(m map[string]bool) map[string]bool {
	if m == nil {
		return nil
	}
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
