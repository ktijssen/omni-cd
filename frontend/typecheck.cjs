// vue-tsc 3.x wraps the JavaScript TypeScript compiler API, which TypeScript 7
// (the native rewrite) no longer ships. Point it at @typescript/typescript6 --
// the TypeScript team's compat shim that re-exports the TS 6 JS API -- so the
// project can stay on typescript@7 for editors and tooling.
//
// Remove this file and go back to `vue-tsc --noEmit` once vue-tsc supports TS 7.
require("vue-tsc").run(require.resolve("@typescript/typescript6/lib/tsc"));
