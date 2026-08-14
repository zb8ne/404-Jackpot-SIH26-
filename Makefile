SHELL := /bin/bash

RPC_URL     ?= http://127.0.0.1:8545
API_URL     ?= http://127.0.0.1:8088
# Anvil prefunded account 0 — the deployer and the registry admin.
DEPLOYER_KEY ?= 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80

CONTRACTS := contracts
BACKEND   := backend
RUN       := .run
DEPLOYMENT := $(CONTRACTS)/deployment.txt

export PATH := $(PATH):$(HOME)/.foundry/bin:$(HOME)/.config/.foundry/bin:$(HOME)/go/bin

.PHONY: demo test build deploy anvil seed backend stop clean tools

## demo: everything, from a clean clone, in one command
demo: stop clean tools build anvil deploy backend seed
	@echo
	@echo "=============================================================="
	@echo " demo is up"
	@echo "   anvil     $(RPC_URL)          (log: $(RUN)/anvil.log)"
	@echo "   backend   $(API_URL)          (log: $(RUN)/backend.log)"
	@echo "   registry  $$(cat $(DEPLOYMENT))"
	@echo "   PDFs      ./demo-files/"
	@echo
	@echo " try it:"
	@echo "   file                                       verdict"
	@echo "   asha-menon-degree.pdf                      VALID"
	@echo "   rahul-iyer-driving-licence.pdf             REVOKED"
	@echo "   asha-menon-birth-certificate-TAMPERED.pdf  TAMPERED"
	@echo "   never-issued-driving-licence.pdf           NOT_ISSUED"
	@echo
	@echo "   curl -s -F file=@demo-files/asha-menon-birth-certificate-TAMPERED.pdf $(API_URL)/verify | jq"
	@echo "   curl -s $(API_URL)/verify/BC-2019-004471 | jq       # what a QR scan hits"
	@echo "   curl -s $(API_URL)/credentials/Asha%20Menon | jq"
	@echo
	@echo " stop with: make stop"
	@echo "=============================================================="

## tools: make sure forge and anvil are on PATH
tools:
	@command -v forge >/dev/null || { \
		echo "forge not found. install foundry:"; \
		echo "  curl -L https://foundry.paradigm.xyz | bash && foundryup"; exit 1; }

## test: the contract test suite
test:
	cd $(CONTRACTS) && forge test -vv

## build: compile contracts and the Go binaries into ./bin
build: test
	@mkdir -p bin
	cd $(BACKEND) && go build -o ../bin/server ./cmd/server && go build -o ../bin/seed ./cmd/seed

## anvil: start a local node in the background
anvil:
	@mkdir -p $(RUN)
	@setsid --fork anvil --silent < /dev/null > $(RUN)/anvil.log 2>&1 & echo $$! > $(RUN)/anvil.pid
	@echo "waiting for anvil..."
	@for i in $$(seq 1 50); do \
		cast block-number --rpc-url $(RPC_URL) >/dev/null 2>&1 && break; \
		sleep 0.2; \
	done
	@cast block-number --rpc-url $(RPC_URL) >/dev/null 2>&1 || { echo "anvil never came up; see $(RUN)/anvil.log"; exit 1; }
	@echo "anvil up on $(RPC_URL) (pid $$(cat $(RUN)/anvil.pid))"

## deploy: deploy the registry and seed the three department roles
deploy:
	cd $(CONTRACTS) && DEPLOYER_KEY=$(DEPLOYER_KEY) \
		forge script script/Deploy.s.sol:Deploy --rpc-url $(RPC_URL) --broadcast
	@echo "registry deployed at $$(cat $(DEPLOYMENT))"

## backend: start the REST API in the background
backend:
	@mkdir -p $(RUN)
	@cd $(BACKEND) && setsid --fork ../bin/server < /dev/null > ../$(RUN)/backend.log 2>&1 & echo $$! > $(RUN)/backend.pid
	@echo "waiting for the backend..."
	@for i in $$(seq 1 100); do \
		curl -sf $(API_URL)/health >/dev/null 2>&1 && break; \
		sleep 0.3; \
	done
	@curl -sf $(API_URL)/health >/dev/null || { echo "backend never came up; see $(RUN)/backend.log"; exit 1; }
	@echo "backend up on $(API_URL)"

## seed: two citizens, three documents each, plus revoked/tampered/never-issued copies
seed:
	./bin/seed -api $(API_URL) -out demo-files

## stop: kill anvil and the backend
stop:
	@-[ -f $(RUN)/backend.pid ] && kill $$(cat $(RUN)/backend.pid) 2>/dev/null || true
	@-[ -f $(RUN)/anvil.pid ] && kill $$(cat $(RUN)/anvil.pid) 2>/dev/null || true
	@-pkill -x server 2>/dev/null || true
	@-pkill -x anvil 2>/dev/null || true
	@-rm -rf $(RUN)
	@echo "stopped"

## clean: throw away all demo state (db, deployed address, generated PDFs)
clean:
	@rm -f $(BACKEND)/credentials.db $(DEPLOYMENT)
	@rm -rf demo-files bin $(CONTRACTS)/broadcast $(CONTRACTS)/cache
	@echo "cleaned"
