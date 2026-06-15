GO = go

.PHONY: lint
lint:
	 golangci-lint --timeout 5m0s run ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix --timeout 5m0s ./...

# ---------------------------------------------------------------------------
# SHS Panel targets
#
# These targets are owned by SHS Studio and are kept additive so upstream
# merges from gameap/gameap stay clean. See docs/architecture/.
# ---------------------------------------------------------------------------

# Roots that must NOT contain any forbidden game-name string. Game-specific
# strings belong only under plugins/ and templates/. Architecture plan §8.2.
SHS_CORE_ROOTS = cmd internal/api internal/app internal/application \
                 internal/domain internal/repositories internal/services \
                 internal/ws migrations openapi web/frontend pkg

# pkg/shspluginsdk is allowed to mention plugin ids; everything else under pkg
# is upstream and must stay game-agnostic. The check below excludes the SDK.

.PHONY: shs-lint
shs-lint: shs-lint-core

.PHONY: shs-lint-core
shs-lint-core:
	@echo "[shs-lint] checking forbidden game-name strings in core..."
	@$(GO) run ./.shs/tools/shslint -config .shs/lint/forbidden-strings.txt

.PHONY: shs-scaffold-check
shs-scaffold-check:
	@echo "[shs-lint] verifying SHS scaffolding directories exist..."
	@for d in internal/shsplugin pkg/shspluginsdk web/shs-theme plugins templates .shs ; do \
		test -d $$d || { echo "missing scaffold dir: $$d"; exit 1; } ; \
	done
	@echo "[shs-lint] scaffolding ok"
