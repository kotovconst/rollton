.PHONY: help fmt fmt-check validate plan apply destroy bootstrap-init bootstrap-apply

ENV ?= prod
ENV_DIR := envs/$(ENV)

help:
	@echo "Targets:"
	@echo "  bootstrap-init   - terraform init in bootstrap/ (local state)"
	@echo "  bootstrap-apply  - terraform apply in bootstrap/ (one-shot)"
	@echo "  fmt              - terraform fmt -recursive across infra/"
	@echo "  fmt-check        - terraform fmt -check -recursive"
	@echo "  validate         - terraform validate in envs/\$$ENV (default: prod)"
	@echo "  plan             - terraform plan in envs/\$$ENV"
	@echo "  apply            - terraform apply in envs/\$$ENV"
	@echo "  destroy          - terraform destroy in envs/\$$ENV"

fmt:
	terraform fmt -recursive .

fmt-check:
	terraform fmt -check -recursive .

validate:
	cd $(ENV_DIR) && terraform init -backend=false -input=false && terraform validate

plan:
	cd $(ENV_DIR) && terraform plan

apply:
	cd $(ENV_DIR) && terraform apply

destroy:
	cd $(ENV_DIR) && terraform destroy

bootstrap-init:
	cd bootstrap && terraform init

bootstrap-apply:
	cd bootstrap && terraform apply
