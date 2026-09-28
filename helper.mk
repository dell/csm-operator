# Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Dell Technologies, Dell and other trademarks are trademarks of Dell Inc.
# or its subsidiaries. Other trademarks may be trademarks of their respective 
# owners.

.PHONY: gen-semver copy-csm-common vendor

gen-semver:
	(cd core; rm -f core_generated.go; go generate)
	go run core/semver/semver.go -f mk > semver.mk

copy-csm-common:
	cp ../csm/config/csm-common.mk .

vendor:
	rm -rf vendor
	GOPRIVATE=github.com go mod vendor
	rm -rf tests/e2e/vendor
	GOPRIVATE=github.com cd tests/e2e && go mod vendor
