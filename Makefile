.PHONY: help bot infra

help:
	@echo "Rollton monorepo — delegate to subprojects:"
	@echo "  make -C bot <target>      # see bot/Makefile"
	@echo "  make -C infra <target>    # see infra/ (placeholder)"

bot:
	@$(MAKE) -C bot $(filter-out $@,$(MAKECMDGOALS))

infra:
	@$(MAKE) -C infra $(filter-out $@,$(MAKECMDGOALS))

%:
	@:
