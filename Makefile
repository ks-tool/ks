LOCALBIN := ./bin
CONTROLLER_GEN := $(LOCALBIN)/controller-gen
DEFAULTER_GEN := $(LOCALBIN)/defaulter-gen
CONVERSION_GEN := $(LOCALBIN)/conversion-gen
GO_LICENSE := $(LOCALBIN)/go-licenses
DIRHACK := hack

.PHONY: controller-gen conversion-gen defaulter-gen generate go-licenses licenses

controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN):
	@$(DIRHACK)/go-install.sh sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5

defaulter-gen: $(DEFAULTER_GEN)
$(DEFAULTER_GEN):
	@$(DIRHACK)/go-install.sh k8s.io/code-generator/cmd/defaulter-gen

conversion-gen: $(CONVERSION_GEN)
$(CONVERSION_GEN):
	@$(DIRHACK)/go-install.sh k8s.io/code-generator/cmd/conversion-gen

go-licenses: $(GO_LICENSE)
$(GO_LICENSE):
	@$(DIRHACK)/go-install.sh github.com/google/go-licenses@latest

generate: controller-gen conversion-gen defaulter-gen
	@$(CONTROLLER_GEN) object:headerFile="$(DIRHACK)/boilerplate.go.txt" paths="./apis/..."
	@$(DEFAULTER_GEN) \
		--go-header-file="$(DIRHACK)/boilerplate.go.txt" \
		--output-file=zz_generated.defaults.go ./apis/yc/...
	@$(CONVERSION_GEN) \
		--go-header-file="$(DIRHACK)/boilerplate.go.txt" \
		--output-file=zz_generated.conversion.go ./apis/yc/...

licenses: go-licenses
	@$(GO_LICENSE) report --template $(DIRHACK)/LICENSES.md.tpl ./... > LICENSES.md
