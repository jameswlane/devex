## [0.20.2](https://github.com/jameswlane/devex/compare/0.20.1...0.20.2) (2025-08-10)

### Bug Fixes

* address critical command injection vulnerabilities ([5460a95](https://github.com/jameswlane/devex/commit/5460a95540048a8ee5f4fd5fccb1db31a330b9fa))
* resolve critical installer issues for improved reliability ([c469e26](https://github.com/jameswlane/devex/commit/c469e26ba53e86b202d57a4189f6b105828eb5d5))
* resolve critical installer issues for improved reliability ([5159092](https://github.com/jameswlane/devex/commit/5159092a0d7c32ac1fcbc4b14c390c9300ed487a))
* resolve Docker security test failures ([50f02a1](https://github.com/jameswlane/devex/commit/50f02a19869cf3e0e087f3cf20941afb8e01d7e0))

## [0.20.1](https://github.com/jameswlane/devex/compare/0.20.0...0.20.1) (2025-08-10)

### Bug Fixes

* vercel issues ([6e0a23c](https://github.com/jameswlane/devex/commit/6e0a23c65a7c32de16a7f8f9f57759767cf82b4b))

## [0.20.0](https://github.com/jameswlane/devex/compare/0.19.0...0.20.0) (2025-08-10)

### Features

* add comprehensive Ghostty terminal emulator with 12 new package managers ([1c567e4](https://github.com/jameswlane/devex/commit/1c567e4454e13a86bd3433862fe521d6342b4119))
* added ghostty to config ([2ada47d](https://github.com/jameswlane/devex/commit/2ada47da9104b121f5cf129bcda98f8bc554d63e))
* implement comprehensive DNF installer for Red Hat-based systems ([10a3fb0](https://github.com/jameswlane/devex/commit/10a3fb08190ec6b5b2dd7834736b380521ff5b9b))
* implement comprehensive stub installer enhancements and structured error handling ([ff330a9](https://github.com/jameswlane/devex/commit/ff330a9c51f7635ca7eff4c9efc157b19c79f0fa))
* implement comprehensive Zypper installer with SUSE platform support ([84e3621](https://github.com/jameswlane/devex/commit/84e3621f04758b69d024c79edc9298d99755f93b))
* implement standardized error handling and background validation ([d368411](https://github.com/jameswlane/devex/commit/d368411599229c9d305c4d272ee74ad1d1bbf188))
* **turbo:** configure TurboRepo for Go CLI and optimize Taskfile ([30ecb74](https://github.com/jameswlane/devex/commit/30ecb7427d4ad750b942c6af9c01ac1141eaebbb))

### Bug Fixes

* **ci:** prevent test hanging in CI with timeouts and progress reporting ([522304b](https://github.com/jameswlane/devex/commit/522304b917ab1630ceb449e4e054b6a082baf455))
* **ci:** resolve Vercel deployment and test failures ([8d8a2ce](https://github.com/jameswlane/devex/commit/8d8a2ce4b7e4b12ddef0b667d9f8e15f240b5de6))
* convert test to ginkgo ([65de7c7](https://github.com/jameswlane/devex/commit/65de7c76f48f2c6e2fa0b23738bc4679cffaec8f))
* **deadlock:** resolve BackgroundValidator mutex deadlock in APT tests ([453475f](https://github.com/jameswlane/devex/commit/453475fed249db697563b168b4939cb011faa213))
* **dnf:** resolve DNF installer test failures and improve YUM fallback ([409a07f](https://github.com/jameswlane/devex/commit/409a07f3f2142b73ecd85ce197dad2d8d57bc811))
* **gitignore:** added vendor ([2305233](https://github.com/jameswlane/devex/commit/230523335963ecc06e6ce5670df5805ca1883c65))
* implement missing BaseInstaller interface methods across all installers ([e63b019](https://github.com/jameswlane/devex/commit/e63b0197a9a1ebd59e1fb9489cf72fc51f587f4e))
* more test fixes ([1e09f9f](https://github.com/jameswlane/devex/commit/1e09f9f5d35e4371e2786f143ec3f97794b99b44))
* more test fixes ([efaafb2](https://github.com/jameswlane/devex/commit/efaafb2268617f1d1a47df02cf8e3a0adbcd5811))
* remove YAML comments to prevent linting issues ([2d405cf](https://github.com/jameswlane/devex/commit/2d405cf58d0494e1f21ebcc0a886597a94dfaac0))
* resolve all 24 golangci-lint security issues ([dacd9a8](https://github.com/jameswlane/devex/commit/dacd9a8fd6c82f00436aaced7e4911f141a824b1))
* resolve APT installer test failures blocking release ([dee2f1e](https://github.com/jameswlane/devex/commit/dee2f1e688095dd294a1bcf2902c992cf961f289))
* test in pipeline ([28c8373](https://github.com/jameswlane/devex/commit/28c8373e47db762911ca86ad46c9d3b7a7252a4f))
* **test:** fixing pipeline ([4d8a3d8](https://github.com/jameswlane/devex/commit/4d8a3d85d0f91dde676846577b5bee77aa748121))
* **test:** remove duplicate Ginkgo suite runner causing pipeline failure ([ed755f1](https://github.com/jameswlane/devex/commit/ed755f18b43ef3f8451b12743f8e3fc95718b62b))

## [0.19.0](https://github.com/jameswlane/devex/compare/0.18.0...0.19.0) (2025-08-06)

### Features

* implement comprehensive list commands with filtering and multip… ([6f053ad](https://github.com/jameswlane/devex/commit/6f053ade6f115df5d4127dee0ca76784808e5b4e))
* implement comprehensive list commands with filtering and multiple output formats ([d656004](https://github.com/jameswlane/devex/commit/d6560042f8044a9fa7a376ac28afd5a671b56efb)), closes [#98](https://github.com/jameswlane/devex/issues/98)

### Bug Fixes

* enhance error handling by using RunE instead of Run ([a1a77f6](https://github.com/jameswlane/devex/commit/a1a77f650c069a9d49e648ff6770fc9078b8c138))
* enhance error handling by using RunE instead of Run ([530207f](https://github.com/jameswlane/devex/commit/530207fdb5a4e6d56eec9ddbaccacaedb7413a0e))
* implement proper installation status checking in list command ([349c991](https://github.com/jameswlane/devex/commit/349c991e30da70c3e9b3f5d36e95152d13c65876))
* remove hardcoded install date from list command ([690ac6e](https://github.com/jameswlane/devex/commit/690ac6e85ddb94321bcec3c7b28c53929bc34997))
* resolve test failures by preventing installation execution during tests ([74aa919](https://github.com/jameswlane/devex/commit/74aa91993b9a793c2d572f13c23330d2f104b315))
* use cmd.OutOrStdout() in completion command for testable output ([3305502](https://github.com/jameswlane/devex/commit/33055024aed364053cd8eaf499c15fff8c552ca7))

### Performance Improvements

* enhance error handling, performance, and code organization ([353192a](https://github.com/jameswlane/devex/commit/353192ad4c606068aeeeac386de9f088f0365c2e))
* optimize list command performance and add comprehensive test coverage ([48b87ce](https://github.com/jameswlane/devex/commit/48b87ce0ce8f102b55d9105725822696c2019bf2))

## [0.18.0](https://github.com/jameswlane/devex/compare/0.17.0...0.18.0) (2025-08-04)

### Features

* add email validation and improve code documentation ([04fc33c](https://github.com/jameswlane/devex/commit/04fc33c953696381868004e7b0fe22eda96f7c88))
* enhance TUI flow and theme system with comprehensive improvements ([5d195ac](https://github.com/jameswlane/devex/commit/5d195ac2a8ecc2a25fb8382871987dba05e0385d))
* enhance TUI flow and theme system with comprehensive improvements ([4620424](https://github.com/jameswlane/devex/commit/46204245f54d257073e4f24dd934dae8b7ca1619))

## [0.17.0](https://github.com/jameswlane/devex/compare/0.16.3...0.17.0) (2025-08-04)

### Features

* add comprehensive theme system test coverage and fix critical issues ([8b78d8a](https://github.com/jameswlane/devex/commit/8b78d8adcf4730637d57edcd647bf2c38bf71eb6))
* implement comprehensive theme system with dynamic selection ([4e2293b](https://github.com/jameswlane/devex/commit/4e2293b0eafafb45feea54dd9a5158bab5f50d75))
* implement comprehensive theme system with dynamic selection ([2eef4a4](https://github.com/jameswlane/devex/commit/2eef4a4d989dc19d3f5c35afacc02b29b7f3c439))

### Bug Fixes

* critical race condition in setup flow installation process ([542e3e4](https://github.com/jameswlane/devex/commit/542e3e4b453433974a430ec3f6d89d17fdf35cbd))

## [0.16.3](https://github.com/jameswlane/devex/compare/0.16.2...0.16.3) (2025-08-03)

### Bug Fixes

* add fallback to direct installer to bypass TUI panic ([bb5eeca](https://github.com/jameswlane/devex/commit/bb5eeca0b1650b42e628bef55f905268dfc17d46))
* add git input handling and panic recovery for streaming installer ([1b79b1f](https://github.com/jameswlane/devex/commit/1b79b1f3ed407fed5fd7e22ff9cb982f2ed10d29))
* correct GPG key download path in APT source installer ([7392e51](https://github.com/jameswlane/devex/commit/7392e518a5e8209a8405d7c160dc225fc7136779))
* correct log inspection path in Docker test script ([9a6db76](https://github.com/jameswlane/devex/commit/9a6db769644d37e254aa3f30bd45753d55306448))
* docs missing DocsLayout component ([c10307b](https://github.com/jameswlane/devex/commit/c10307bfc6a3a7a38978fc67f2ff3b29a44db9b5))
* docs missing DocsLayout component ([3dcaaca](https://github.com/jameswlane/devex/commit/3dcaaca4cd596a2dc471bb47836af615ef5252eb))
* fix test ([eba0d02](https://github.com/jameswlane/devex/commit/eba0d02e76eeddbae550016e8712cf2b0b6792b1))
* improve Docker installation and user detection in containers ([93d7d6a](https://github.com/jameswlane/devex/commit/93d7d6acdf799705227e80239523fd12b9a3cbc1))
* resolve multiple TUI installer issues and panics ([eb07a0b](https://github.com/jameswlane/devex/commit/eb07a0b7bedbfda5f66ca8480cff99e3e67f30e8))
* resolve streaming installer panic and git input issues ([8edaaa6](https://github.com/jameswlane/devex/commit/8edaaa6e840657733ee2ac0447e241a79f5b585a))

## [0.16.2](https://github.com/jameswlane/devex/compare/0.16.1...0.16.2) (2025-08-02)

### Bug Fixes

* implement automatic Docker service setup and permission handling ([5648975](https://github.com/jameswlane/devex/commit/5648975f791fc431c5fd6b40bbaebebd9b90fd15))
* improve interactive mode detection for guided setup ([af41bf8](https://github.com/jameswlane/devex/commit/af41bf831e3c11fe91da9e76e4ae87c98ad2dde5))
* improve interactive mode detection for guided setup ([62a991c](https://github.com/jameswlane/devex/commit/62a991c35be4e2b876609107dd114f1728ecf59c))

## [0.16.1](https://github.com/jameswlane/devex/compare/0.16.0...0.16.1) (2025-08-02)

### Bug Fixes

* improve Docker installation error handling and APT verification ([a8ec52d](https://github.com/jameswlane/devex/commit/a8ec52de2197c235cb94a4bc66a447bd62668279))

## [0.16.0](https://github.com/jameswlane/devex/compare/0.15.1...0.16.0) (2025-08-02)

### Features

* implement comprehensive installer restructuring with theme management ([52fdc34](https://github.com/jameswlane/devex/commit/52fdc34b1a498f48d37ab05ff09a04aa77d7f9ba))
* implement comprehensive security hardening and test coverage ([0b04422](https://github.com/jameswlane/devex/commit/0b044222a913230dec6bfd9f41172a60cc4848d0))

### Bug Fixes

* resolve golangci-lint noctx violations in theme_manager.go ([8381e5e](https://github.com/jameswlane/devex/commit/8381e5e4c19328322a127f525db12921c4b08a2d))
* resolve installer issues in automated setup ([d20f11f](https://github.com/jameswlane/devex/commit/d20f11faf6e1766909794eae535c77acb294cfd2))
* resolve test failures and Ginkgo suite conflicts ([fd45267](https://github.com/jameswlane/devex/commit/fd45267a7c9f6f7ef6b2f88836f5a98ef26c4146))

## [0.15.1](https://github.com/jameswlane/devex/compare/0.15.0...0.15.1) (2025-07-31)

### Bug Fixes

* resolve Windows cross-compilation issue with syscall.SysProcAttr ([db668c3](https://github.com/jameswlane/devex/commit/db668c356a3052000e2e8743d026f9ec0eedfbe3))

## [0.15.0](https://github.com/jameswlane/devex/compare/0.14.0...0.15.0) (2025-07-31)

### Features

* implement comprehensive GPG key fingerprint validation and enhance security ([aa6636c](https://github.com/jameswlane/devex/commit/aa6636cf77934f60218a91cd2ce8eac228bde3c6))
* **tui:** working on a better installer ([b461023](https://github.com/jameswlane/devex/commit/b461023f9b849a35facd953b905f377acf129757))
* **tui:** working on a better installer ([d4c674e](https://github.com/jameswlane/devex/commit/d4c674e17d3393de00ddd35b9341474d56b0a8ab))

### Bug Fixes

* critical security vulnerabilities in installer and TUI ([2be1448](https://github.com/jameswlane/devex/commit/2be1448eef17921c185d7986cce2b93df3506ad0))
* resolve all critical security vulnerabilities and code quality issues ([f446fbc](https://github.com/jameswlane/devex/commit/f446fbc7357c37a7ad05f4f4ca9ab1bf69ed2808))

## [0.14.0](https://github.com/jameswlane/devex/compare/0.13.1...0.14.0) (2025-07-29)

### Features

* implement streaming installer with real-time installation progress ([a3a9096](https://github.com/jameswlane/devex/commit/a3a90963761123ccde1c3bb9def93716b4ad01bf))

### Bug Fixes

* correct Claude Review action configuration ([fda0fa3](https://github.com/jameswlane/devex/commit/fda0fa3af67c1c7f36e2355cfd938aae30cd3268))
* improve issue workflow configurations ([cbc14a0](https://github.com/jameswlane/devex/commit/cbc14a08aff491f63a3b38e188f5af250f06a1df))

## [0.13.1](https://github.com/jameswlane/devex/compare/0.13.0...0.13.1) (2025-07-28)

### Bug Fixes

* enhance Docker validation and user guidance ([a58674d](https://github.com/jameswlane/devex/commit/a58674db310ec33448b8ff86b158566f45440fa2))

## [0.13.0](https://github.com/jameswlane/devex/compare/0.12.0...0.13.0) (2025-07-28)

### Features

* create dedicated Docker installer utility ([b927cb5](https://github.com/jameswlane/devex/commit/b927cb5bcc8ea58987ed03f7254be011016ad156))

## [0.12.0](https://github.com/jameswlane/devex/compare/0.11.0...0.12.0) (2025-07-28)

### Features

* implement Quick Win improvements for enhanced user experience ([fd43161](https://github.com/jameswlane/devex/commit/fd4316136f13d177ae2ca25e26deb825b330bc97))

## [0.11.0](https://github.com/jameswlane/devex/compare/0.10.0...0.11.0) (2025-07-28)

### Features

* improve shell change handling and add devex shell command ([c824afc](https://github.com/jameswlane/devex/commit/c824afc2696cb3db93d4a7f44cae94774d5a45cd))

### Bug Fixes

* update Docker configuration to new cross-platform format ([368e2b4](https://github.com/jameswlane/devex/commit/368e2b4488aebd3f462c1341f978b4a8a4605032))

## [0.10.0](https://github.com/jameswlane/devex/compare/0.9.1...0.10.0) (2025-07-27)

### Features

* implement centralized logging architecture for clean TUI ([ffc7f16](https://github.com/jameswlane/devex/commit/ffc7f162d8a563fe3dc5a4a928e8a737c05d8b05))

### Bug Fixes

* resolve test failures in log and db packages ([f1220e5](https://github.com/jameswlane/devex/commit/f1220e5fed14e845b1a74c364171a78051f7b2a8))

## [0.9.1](https://github.com/jameswlane/devex/compare/0.9.0...0.9.1) (2025-07-27)

### Bug Fixes

* improve logging and error handling for guided setup ([84e9cbf](https://github.com/jameswlane/devex/commit/84e9cbfe5573fc61e952e9a25893df06894e66ca))

## [0.9.0](https://github.com/jameswlane/devex/compare/0.8.0...0.9.0) (2025-07-27)

### Features

* add comprehensive bash and fish shell configurations ([564dad7](https://github.com/jameswlane/devex/commit/564dad7bb6f3cffd71d7d5b900ed08efa09eb6ad))

## [0.8.0](https://github.com/jameswlane/devex/compare/0.7.1...0.8.0) (2025-07-27)

### Features

* add shell selection to guided setup ([40033f2](https://github.com/jameswlane/devex/commit/40033f255c60270e5d0d1da0b23f9c4ec1dc14ec))
* implement guided setup with dependency ordering ([9da10a1](https://github.com/jameswlane/devex/commit/9da10a1b781c9237dc41182990772973bd4a18d3))

## [0.7.1](https://github.com/jameswlane/devex/compare/0.7.0...0.7.1) (2025-07-27)

### Bug Fixes

* add proper dependency verification and graceful error handling ([9eb745e](https://github.com/jameswlane/devex/commit/9eb745e684ca7a6f87f8abfe4a12d93f3d256ea6))

## [0.7.0](https://github.com/jameswlane/devex/compare/0.6.0...0.7.0) (2025-07-27)

### Features

* add Configuration Management System to roadmap ([bdd232d](https://github.com/jameswlane/devex/commit/bdd232d7267bf28f2923e6270762b0e9ac754177))
* implement interactive guided setup with bubbletea ([e9fe4f2](https://github.com/jameswlane/devex/commit/e9fe4f253d30ebbf9feddce40a8e0ac2ac376f09))

## [0.6.0](https://github.com/jameswlane/devex/compare/0.5.1...0.6.0) (2025-07-27)

### Features

* defer zsh shell switching to final installation step ([b32c5ec](https://github.com/jameswlane/devex/commit/b32c5ec45e9225b1575498b3618d2547a5cdf4e3))

### Bug Fixes

* **hero:** removed wget in favor of curl ([0e7c5fd](https://github.com/jameswlane/devex/commit/0e7c5fdad9309ff0ec957e2c03977a732215a3cb))
* use exec.CommandContext to address linting errors ([ad88ef0](https://github.com/jameswlane/devex/commit/ad88ef09f161f04e3b3b00214de99c8ec464606a))

## [0.5.1](https://github.com/jameswlane/devex/compare/0.5.0...0.5.1) (2025-07-27)

### Bug Fixes

* resolve critical installation issues ([384f301](https://github.com/jameswlane/devex/commit/384f301af93f6ad16de6a3eaa100207e83862556))

## [0.5.0](https://github.com/jameswlane/devex/compare/0.4.0...0.5.0) (2025-07-27)

### Features

* **web:** add curl alternative to installation commands ([eb4dd7b](https://github.com/jameswlane/devex/commit/eb4dd7b3c8c7a2ddb9d398698b1c394c89444c7b))

### Bug Fixes

* **fumadocs:** fixed meta.json ([867567d](https://github.com/jameswlane/devex/commit/867567d349590af289a5872480de8cae007faa87))
* **installer:** handle missing wget/curl gracefully ([a0df510](https://github.com/jameswlane/devex/commit/a0df5101020edc864d43f8278f014e217169444a))

## [0.4.0](https://github.com/jameswlane/devex/compare/0.3.1...0.4.0) (2025-07-27)

### Features

* complete monorepo restructuring and fix compilation errors ([3325a16](https://github.com/jameswlane/devex/commit/3325a164599d912595f8f4c781ba0946d5a47ad3))
* **docs:** add new documentation app with MDX and Next.js ([7b53264](https://github.com/jameswlane/devex/commit/7b5326443af0370b67732f6d50664fcd0e656cb3))
* implement cross-platform installer system and remove legacy compatibility ([efd4c56](https://github.com/jameswlane/devex/commit/efd4c56eed0949af95a4135b7e51c08ca491adbe))

### Bug Fixes

* **github-action:** fixing action config ([f4b7422](https://github.com/jameswlane/devex/commit/f4b7422ea2a95cbe6e87b9b9699f04dda7161cf8))
* **go:** updated dependencies ([6673f1c](https://github.com/jameswlane/devex/commit/6673f1ce2fdb68194ceec3a2f4c79ee02833df55))
* **go:** updated go ([1bad452](https://github.com/jameswlane/devex/commit/1bad452889a08e18d6044d9b790aa5bb2fe55420))
* **lint:** resolve lint issues and enhance security ([41f0261](https://github.com/jameswlane/devex/commit/41f0261564a72eaffacbb9431e7437b1d0e1f18d))
* **noctx:** complete noctx linter fixes for database and exec operations ([2b52b21](https://github.com/jameswlane/devex/commit/2b52b21d07f5512fc8e9d0adf92a2671c96b6443))
* **noctx:** use context-aware database operations ([6bce15b](https://github.com/jameswlane/devex/commit/6bce15b5fb30a09afac3dc8a05f0bc0c32facc71))
* **staticcheck:** simplify embedded field access in db.go ([bbe46fd](https://github.com/jameswlane/devex/commit/bbe46fd334509f0e0ea30be94063132f21dfb7c5))
* **turbo:** fixing turbo config ([733e435](https://github.com/jameswlane/devex/commit/733e4351c23a08b39f2acdca6ad88aaca298d9ba))
* **turborepo:** more tweaks to turbo config ([f0cd310](https://github.com/jameswlane/devex/commit/f0cd3101dea08cfe61635b9baa6233e2360c8a4c))
* **web:** resolve TypeScript build errors in ToolSearch component ([78ab20c](https://github.com/jameswlane/devex/commit/78ab20c319eb242dff40db659c6bb3eeeee14f60))

## [0.3.1](https://github.com/jameswlane/devex/compare/0.3.0...0.3.1) (2025-01-08)

### Bug Fixes

* **ci:** build pipeline ([aaad350](https://github.com/jameswlane/devex/commit/aaad350fa009d47140e9aeaf25980686fdaab575))
* **ci:** ginkgo in action ([b09750d](https://github.com/jameswlane/devex/commit/b09750d5cbd0ef7b13b9a2001040997e4930aaa3))

## [0.3.0](https://github.com/jameswlane/devex/compare/0.2.4...0.3.0) (2025-01-08)

### Features

* **rewrite:** large rewrite with some testing added ([dc6a5e2](https://github.com/jameswlane/devex/commit/dc6a5e23af5c5eb3fdc4a351f6edfb557e0f6f33))

## [0.2.4](https://github.com/jameswlane/devex/compare/0.2.3...0.2.4) (2025-01-03)

### Bug Fixes

* **refactor:** some refactoring of the code ([c102141](https://github.com/jameswlane/devex/commit/c102141588bee3a121a6d66c47f0d2eadaae0da7))

## [0.2.3](https://github.com/jameswlane/devex/compare/0.2.2...0.2.3) (2025-01-02)

### Bug Fixes

* **filesystem:** filesystem rewrite ([08bf3bb](https://github.com/jameswlane/devex/commit/08bf3bbc46b1c815dc3c26e94d473e3c5b412493))

## [0.2.2](https://github.com/jameswlane/devex/compare/0.2.1...0.2.2) (2025-01-01)

### Bug Fixes

* **filesystem:** setting up wrappers for easy mocking and testing ([af6c00b](https://github.com/jameswlane/devex/commit/af6c00bd39b8005b914db1a948bc55e15378f834))

## [0.2.1](https://github.com/jameswlane/devex/compare/0.2.0...0.2.1) (2024-12-31)

### Bug Fixes

* **docs:** fixing docs ([eec71b5](https://github.com/jameswlane/devex/commit/eec71b5a1a9157d74fb4fb2d77720643f50b1eb7))

## [0.2.0](https://github.com/jameswlane/devex/compare/0.1.8...0.2.0) (2024-12-28)

### Features

* **installer:** continued reworking of the installer ([b6f7e3a](https://github.com/jameswlane/devex/commit/b6f7e3ada9749116a79e006ba71d52be8ebf2235))

## [0.1.8](https://github.com/jameswlane/devex/compare/0.1.7...0.1.8) (2024-12-25)

### Bug Fixes

* **defaults:** updated default apps, added more tested apps ([8b293e1](https://github.com/jameswlane/devex/commit/8b293e139d7bdff35647a755680d2504218cd0b3))

## [0.1.7](https://github.com/jameswlane/devex/compare/0.1.6...0.1.7) (2024-12-24)

### Bug Fixes

* **installer:** fixed the installer, added gpg and apt source ([6b3a432](https://github.com/jameswlane/devex/commit/6b3a4322f84b4f6cf3e1800347164b56a5cf1506))

## [0.1.6](https://github.com/jameswlane/devex/compare/0.1.5...0.1.6) (2024-12-23)

### Bug Fixes

* **datastore:** finished migration and datastore versioning ([bb1bfb9](https://github.com/jameswlane/devex/commit/bb1bfb9c659e403b4b7ec210fc7d65bf355bf06d))

## [0.1.5](https://github.com/jameswlane/devex/compare/0.1.4...0.1.5) (2024-12-22)

### Bug Fixes

* **everything:** large rewrite nothing to see here ([19a7e42](https://github.com/jameswlane/devex/commit/19a7e42272fd424eea7bbfdb173bae23f3173198))
