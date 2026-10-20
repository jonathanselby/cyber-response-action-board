# CRAB build evidence — 0.0.10-dev

## Checkpoint

Slice 9 — model assistance.

Substantive source head before promotion: `dbe6798508007f02d8938b838c159bc117b356ff`.

Source CI: GitHub Actions run #111 / `39380512478` — module contract, gofmt, vet, full race/PostgreSQL suite, linux/amd64 build, linux/arm64 build and checkpoint acceptance all passed.

No release or tag is created at this development checkpoint.

## Built

- Five small static CRAB draft specifications: retrieval summary, situation report, board update, notification language and PIR summary.
- Each specification has a prompt version, purpose, allowed explicit lane(s), source-policy rule, untrusted output type, citation requirement and bounded output size.
- Explicit `simple`, `standard` and `complex` lanes. The caller selects the lane; there is no hidden classifier or automatic provider selection.
- Provider/model routing derived from the Garage three-lane harness contract.
- Local Ollama adapter and managed Azure OpenAI/Foundry, Amazon Bedrock and Google Vertex adapters using standard-library HTTP; no cloud SDK or new direct dependency was added.
- Deterministic retrieval runs before model generation. Only already-authorised citations can enter the prompt.
- Managed lanes require the sensitivity policy and the separate `model_send` authorisation before provider I/O.
- Model prompt provenance records spec ID, prompt version, selected provider/model, prompt SHA-256 and stable citation IDs; execution traces add latency and provider-reported token counts.
- Model outputs are always marked untrusted. `validated=true` means only that deterministic structural checks passed; it is not human approval and does not create effective governed state.
- Empty, oversized or NUL-containing outputs fail validation visibly while retaining returned text, authorised sources and trace metadata.
- Provider failure retains authorised sources and failure trace. There is no automatic reroute or failover to a different lane/provider.
- Model concurrency is bounded. When capacity is full, new model generation is shed immediately while deterministic retrieval/core work remains available.
- Provider HTTP response bodies are not copied into managed-provider error strings, reducing the chance of remote sensitive content entering logs/error surfaces.
- Live/exercise incident mode is explicit at the model policy boundary.

## Security and authority proof

Tests prove:

- a managed lane denied by sensitivity policy makes zero provider calls;
- a managed lane without the separate `model_send` permission makes zero provider calls;
- a failed configured provider is not rerouted;
- queue saturation sheds model work while deterministic retrieval still succeeds;
- unavailable or invalid model output retains authorised source provenance and never becomes governed state;
- local and managed provider wire contracts are exercised with local HTTP test servers, not live cloud credentials;
- the four provider adapters and router require explicit configured lanes and registered providers;
- prompt construction is deterministic for the same versioned spec, instruction and stable citations.

## Deliberate boundary

This checkpoint builds the model-assistance kernel and provider adapters. It does not fake deployment credentials, live cloud-account integration, operator-facing model configuration, browser model routes, autonomous agents, automatic complexity classification, automatic failover, a model queue service, an evaluation service, embeddings or a vector database.

The deterministic incident system remains functional without any model provider.
