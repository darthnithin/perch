.PHONY: build test vet check demo terminal-test chip-test shell-test cancel-test mcp-test nrp-test nrp-ui-test release release-check notices-check homebrew-test

build:
	go build -trimpath -o bin/perch ./cmd/perch

test:
	go test -race ./...

vet:
	go vet ./...

check: test vet
	@test -z "$$(gofmt -l cmd internal assets/bird tools/chip-preview testdata/hummingbird/fixtures.go testdata/nautilus/fixtures.go)"

chip-test:
	go build -trimpath -o bin/chip-preview ./tools/chip-preview
	.venv/bin/python tools/test_chip_preview.py

demo: build
	./bin/perch --demo

shell-test: build
	python3 tools/test_allocation_shell_probe.py
	python3 tools/test_shell.py
	.venv/bin/python tools/test_shell_ui.py

cancel-test: build
	python3 tools/test_cancel.py
	.venv/bin/python tools/test_cancel_ui.py

terminal-test: build
	.venv/bin/python tools/test_terminal_emulator.py
	.venv/bin/python tools/test_terminal.py
	.venv/bin/python tools/test_mouse.py
	.venv/bin/python tools/test_hover.py
	.venv/bin/python tools/test_footer.py
	.venv/bin/python tools/test_glance.py
	.venv/bin/python tools/test_pending_guidance.py
	.venv/bin/python tools/test_job_gpus.py
	.venv/bin/python tools/test_empty_workspace.py
	.venv/bin/python tools/test_compact.py
	.venv/bin/python tools/test_help.py
	.venv/bin/python tools/test_palette.py
	.venv/bin/python tools/test_job_actions.py
	.venv/bin/python tools/test_job_sort.py
	.venv/bin/python tools/test_activity.py
	.venv/bin/python tools/test_range_events.py
	.venv/bin/python tools/test_warnings.py
	.venv/bin/python tools/test_setup.py
	.venv/bin/python tools/test_logs.py
	.venv/bin/python tools/test_static_readers.py
	.venv/bin/python tools/test_script_outcomes.py
	.venv/bin/python tools/test_script_search.py
	.venv/bin/python tools/test_log_search.py
	.venv/bin/python tools/test_log_retention.py
	.venv/bin/python tools/test_diagnosis.py
	.venv/bin/python tools/test_resource_steps.py
	.venv/bin/python tools/test_live_steps.py
	.venv/bin/python tools/test_launch.py
	.venv/bin/python tools/test_software.py
	.venv/bin/python tools/test_module_copy.py
	python3 tools/test_shell.py
	python3 tools/test_cancel.py
	.venv/bin/python tools/test_cancel_ui.py
	.venv/bin/python tools/test_shell_ui.py
	.venv/bin/python tools/test_failure_terminal.py
	.venv/bin/python tools/test_receipts.py
	.venv/bin/python tools/test_array_gap_terminal.py
	.venv/bin/python tools/test_array_copy.py
	.venv/bin/python tools/test_large_array.py
	.venv/bin/python tools/test_preferences.py
	.venv/bin/python tools/test_cache.py
	.venv/bin/python tools/test_color_mode.py
	.venv/bin/python tools/test_waits.py
	.venv/bin/python tools/test_priority.py
	.venv/bin/python tools/test_nodes.py
	.venv/bin/python tools/test_report_return.py
	.venv/bin/python tools/test_diagnostics.py
	.venv/bin/python tools/test_attempts.py

release:
	sh tools/release.sh "$${VERSION:-dev}"

mcp-test: build
	python3 tools/test_mcp.py

release-check: release
	python3 tools/test_release_archives.py "$${VERSION:-dev}"
	python3 tools/test_notice_validation.py "dist/perch-$${VERSION:-dev}-darwin-arm64.tar.gz"

notices-check:
	python3 tools/notices.py --check

homebrew-test:
	python3 tools/test_homebrew_generator.py

nrp-test: build
	python3 tools/test_nrp_prepare.py
	python3 tools/test_nrp.py

nrp-ui-test: build
	.venv/bin/python tools/test_nrp_metrics.py
	.venv/bin/python tools/test_nrp_quota.py
	.venv/bin/python tools/test_nrp_ui.py
	.venv/bin/python tools/test_nrp_hover.py
	.venv/bin/python tools/test_nrp_footer.py
	.venv/bin/python tools/test_nrp_logs_ui.py
	.venv/bin/python tools/test_nrp.py --ui
