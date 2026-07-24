// Compile-only Phase 0 example. The host supplies schemas and binds the
// declared directory.lookup capability; this source receives no ambient I/O.
const A = require("tinyidp").v1;

module.exports = A.program("phase0-example", program => {
  program.capabilities({
    "directory.lookup": {version: 1},
  });

  const normalize = program.lambda("signup.normalize", {
    input: "signupStartInput",
    output: "signupResult",
    outcomes: ["complete"],
    effects: [],
    capabilities: [],
    timeoutMs: 100,
    maxCapabilityCalls: 0,
    maxOutputBytes: 4096,
    run: ctx => A.result.complete({email: ctx.input.email.trim().toLowerCase()}),
  });

  const lookup = program.lambda("signup.lookup", {
    input: "signupStartInput",
    output: "signupResult",
    outcomes: ["complete", "deny"],
    effects: ["read"],
    capabilities: ["directory.lookup"],
    timeoutMs: 250,
    maxCapabilityCalls: 1,
    maxOutputBytes: 4096,
    run: async ctx => {
      const found = await ctx.cap.directory.lookup({email: ctx.input.email});
      return found.allowed ? A.result.complete(found) : A.result.deny("not_allowed");
    },
  });

  program.workflow("signup-normalize", {
    version: 1,
    entry: "normalize",
    handlers: {normalize},
    edges: [],
  });
  program.workflow("signup-lookup", {
    version: 1,
    entry: "lookup",
    handlers: {lookup},
    edges: [],
  });
});
