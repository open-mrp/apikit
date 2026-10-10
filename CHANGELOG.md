# Changelog

## [0.1.1](https://github.com/open-mrp/apikit/compare/v0.1.0...v0.1.1) (2026-10-10)


### Bug Fixes

* **validate:** check the fields inside optional sections without registering each type ([#3](https://github.com/open-mrp/apikit/issues/3)) ([dfbb096](https://github.com/open-mrp/apikit/commit/dfbb096887a748dc21ae3f9c7f1a9ab4210ffeb6))

## 0.1.0 (2026-10-10)


### ⚠ BREAKING CHANGES

* **apierror:** restore the top-level param on the error object

### Features

* **apierror:** add the forge.1 error object with a code registry ([673feec](https://github.com/open-mrp/apikit/commit/673feeca558f0f566f95195f8c1bb63151d152da))
* **apierror:** let a caller name the param after building an error ([34ba8d2](https://github.com/open-mrp/apikit/commit/34ba8d24797044b474cab469220124005a40461c))
* **apierror:** restore the top-level param on the error object ([95aa37a](https://github.com/open-mrp/apikit/commit/95aa37aa8d7e35c9a0bde2cf7ad1303deed9a6ae))
* **appctx:** add request context keys and a generic caller identity ([069d191](https://github.com/open-mrp/apikit/commit/069d191da1be08f85203755ee6c0bd8f566fcd9b))
* **appctx:** add the API version context key ([08bbf80](https://github.com/open-mrp/apikit/commit/08bbf80be754cd00328f19937fa11687c4b6391a))
* **appctx:** add the request log carried through a request ([74676ae](https://github.com/open-mrp/apikit/commit/74676aeb40ecc105758a99655a7ef039f74a0715))
* **blobstore:** add immutable JSON documents in object storage ([0fea346](https://github.com/open-mrp/apikit/commit/0fea34634a532a6964ae471a1318b22f48c87886))
* **cache:** add scoped read-through caching over memory or Redis ([91c9c3a](https://github.com/open-mrp/apikit/commit/91c9c3af4be6b1cb0511eeafa50e6e396c078941))
* **cloud/s3:** add an S3 client with tracing and a test stub ([f4e96c2](https://github.com/open-mrp/apikit/commit/f4e96c2e1839506e3330277fc5256e5119177a2e))
* **cloud/sqs:** add a long-polling SQS consumer client ([ecda52e](https://github.com/open-mrp/apikit/commit/ecda52e956bff2499f8ad90a70df3de3663b35f4))
* **conformance:** check endpoints against the forge.1 conventions ([2067219](https://github.com/open-mrp/apikit/commit/206721959769e9adf0ec6e1612a30443714b9823))
* **crypto:** add AES-GCM envelopes, bcrypt, HMAC and random strings ([7c25651](https://github.com/open-mrp/apikit/commit/7c256513d786d0b8b04f32d9ad67559ae76c7e79))
* **db:** add pooled connections, transactions, null types and SQL error mapping ([956bd1d](https://github.com/open-mrp/apikit/commit/956bd1da44ab842f6c7d2701bf67d083800bd6a9))
* **endpoint:** add typed API endpoints and their request pipeline ([089ec03](https://github.com/open-mrp/apikit/commit/089ec035eb64a51c0b2760788dd05253011ca1b6))
* **example:** add schema examples for generated API docs ([738f9e6](https://github.com/open-mrp/apikit/commit/738f9e6bdae3d934f60a93bbe3a074e4b86ddfe5))
* **field:** add Optional and Clearable request presence types ([b083f83](https://github.com/open-mrp/apikit/commit/b083f838a1bfb96f884d2a0d1d32ebe558da6663))
* **fuzzy:** add Levenshtein distance and typo matching ([0601eec](https://github.com/open-mrp/apikit/commit/0601eecd4b54dbebb851dcf10167ac54ad3392d7))
* **id:** add prefixed ID generation and prefix composition ([c049c21](https://github.com/open-mrp/apikit/commit/c049c214cbf547c6560adbfe7da6c7fdd6cb6888))
* **idempotency:** add request hashing and the Store interface ([f4bfa3b](https://github.com/open-mrp/apikit/commit/f4bfa3b386c23b6d5c35ea145bffcd2e5470bc3b))
* **include:** add the expandable-include registry and resolver ([02861b9](https://github.com/open-mrp/apikit/commit/02861b99b8bf2fbb8da95058ce97b03f558f16c9))
* **lease:** add a SQL-backed single-holder lease for periodic tasks ([d35cf3d](https://github.com/open-mrp/apikit/commit/d35cf3dbc834048a9eb4b9f0c3da1af83e307493))
* **logging:** add the shared attributes of a canonical log line ([57223f4](https://github.com/open-mrp/apikit/commit/57223f409f1b1218431af878665415495eef1e3d))
* **metadata:** add the client-owned metadata map and its limits ([306ccee](https://github.com/open-mrp/apikit/commit/306ccee1a916c0bd06e828663558b01abebeec02))
* **middleware:** add CORS, security headers, recover, tracing, IP block and version middleware ([278d384](https://github.com/open-mrp/apikit/commit/278d3841b359b65171e8918f6096223acbaabf36))
* **middleware:** add Idempotency-Key handling ([cb1833b](https://github.com/open-mrp/apikit/commit/cb1833b2966b136080e13ad63fe8e6269bb47211))
* **middleware:** add rate limiting with RateLimit-* headers ([0c3ff30](https://github.com/open-mrp/apikit/commit/0c3ff30742a235b4b0fe2b55c233adb30282c156))
* **middleware:** add request logging and canonical log lines ([2396d8d](https://github.com/open-mrp/apikit/commit/2396d8d63873f0c5a98c7b82cd1449eb15139875))
* **object:** add list, deleted stub and Money shapes ([b3c3105](https://github.com/open-mrp/apikit/commit/b3c3105eeb0d22e4b8aef14d9b5087253586d549))
* **object:** add the async job a long-running action returns ([5aeb4ba](https://github.com/open-mrp/apikit/commit/5aeb4bad6516a86668dce21cbef01b0b0498ec49))
* **object:** add the object type every resource reports ([70ec95d](https://github.com/open-mrp/apikit/commit/70ec95deef694d0be6e7da53dd90738adc9c7de6))
* **openapi:** export RouteSegmentBefore for path parameter hooks ([9c5d02d](https://github.com/open-mrp/apikit/commit/9c5d02d64686052758420164c25167257599e88f))
* **openapi:** generate OpenAPI specs, Stainless configs and agent tools from endpoints ([0671ffa](https://github.com/open-mrp/apikit/commit/0671ffadaa76312f11262a282e52a509871c78bc))
* **pagination:** add signed keyset cursors and page building ([136da2e](https://github.com/open-mrp/apikit/commit/136da2e96abe223b4bd60f077d4794ce59302682))
* **ptrutil:** add pointer helpers ([254d19a](https://github.com/open-mrp/apikit/commit/254d19a5bc7708c8e533114e02391b5a860d7761))
* **querytag:** add SQLCommenter query tags carried on a context ([f3d61dc](https://github.com/open-mrp/apikit/commit/f3d61dce3b7ffedafa4e80c1702fc6586c09ab04))
* **ratelimit:** add request limiters in memory and on Redis ([62e3885](https://github.com/open-mrp/apikit/commit/62e38851b62c2a5f9a9c291893ad8b34ff59f892))
* **redact:** add log redaction driven by sensitive struct tags ([9331f17](https://github.com/open-mrp/apikit/commit/9331f170364dcb54fa06377d9bf7e400f73d9f2b))
* **retry:** add retry with exponential backoff and jitter ([9034480](https://github.com/open-mrp/apikit/commit/90344800201b6e07f13d37afff740f75e407228e))
* **router:** add Match for finding a request's route pattern ([0e03d85](https://github.com/open-mrp/apikit/commit/0e03d85a4488654e18d7d64a87306aa1e72979c2))
* **router:** add the route matcher and endpoint registry ([c89fa87](https://github.com/open-mrp/apikit/commit/c89fa87d3bc6fe6a8c1eaf7ea5f4cdb810749029))
* **safeconv:** add overflow-safe integer conversions ([154dcfe](https://github.com/open-mrp/apikit/commit/154dcfe76a2826fcbb01ed07e429d2fd74061e44))
* **sensitive:** add response redaction by registered data class ([fb5a8cf](https://github.com/open-mrp/apikit/commit/fb5a8cfc60fe8f30038cfc17c729d84dc6cbc62c))
* **timeutil:** add Date for forge.1 _on business-day fields ([1f7086e](https://github.com/open-mrp/apikit/commit/1f7086e05868ba82f18ac498f60dccbb70be9234))
* **timeutil:** add timestamp parsing, time zones and context budgets ([dc08804](https://github.com/open-mrp/apikit/commit/dc088042c9c707e725d349a7697933f2b068e986))
* **tracing:** add OpenTelemetry setup, HTTP spans and API error recording ([633bae2](https://github.com/open-mrp/apikit/commit/633bae25670f4685170a8d4567f626afaf651fea))
* **transport:** add request binding and JSON responses ([2a5e136](https://github.com/open-mrp/apikit/commit/2a5e1364e372dea70787c71e46d50f1d7efa4fb6))
* **txaudit:** audit transaction callbacks for effects a rollback would not undo ([3f4a4c3](https://github.com/open-mrp/apikit/commit/3f4a4c3a52b01ae2ebbbd0fdec248bf8dd576f71))
* **validate:** add struct-tag validation that reports every failing field ([185af47](https://github.com/open-mrp/apikit/commit/185af4722bfbbac274587e160902c91bda3702a2))
* **version:** add registered API versions and the transformer engine ([55f700d](https://github.com/open-mrp/apikit/commit/55f700d215cac5ad25bfee22c90e01249722dfbc))


### Bug Fixes

* **openapi:** note why reading the Stainless config by path is safe ([9416616](https://github.com/open-mrp/apikit/commit/941661622f4e0c804ac630462620592fca164bda))
* **transport:** report a malformed JSON body as a 400 ([db0362c](https://github.com/open-mrp/apikit/commit/db0362cb0203cfdb4e266c48b48ba8ea938bfde8))


### Code Refactoring

* **object:** remove has_next_page and has_previous_page from PageInfo ([6bc4a8e](https://github.com/open-mrp/apikit/commit/6bc4a8e2944e78e7ed0b594482acf69f6bd462b6))


### Documentation

* **object:** write response type comments for API readers ([3998cfe](https://github.com/open-mrp/apikit/commit/3998cfe67003f20e6e14534d5b5c9c3377e03043))
