SHELL := /bin/bash

RPC_URL     ?= http://127.0.0.1:8545
API_URL     ?= http://127.0.0.1:8088
WEB_URL     ?= http://127.0.0.1:5173
# Anvil prefunded account 0 — the deployer and the registry admin.
DEPLOYER_KEY ?= 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80

CONTRACTS := contracts
BACKEND   := backend
FRONTEND  := frontend
RUN       := .run
DEPLOYMENT := $(CONTRACTS)/deployment.txt

SEED_BIRTH_EMAIL     ?= birth-admin@example.gov
SEED_TRANSPORT_EMAIL ?= transport-admin@example.gov
SEED_EDUCATION_EMAIL ?= education-admin@example.gov

export PATH := $(PATH):$(HOME)/.foundry/bin:$(HOME)/.config/.foundry/bin:$(HOME)/go/bin

.PHONY: demo test build deploy anvil profiles seed backend frontend stop clean tools

## demo: everything, from a clean clone, in one command
demo: stop clean tools build anvil deploy backend profiles seed frontend
	@echo
	@echo "=============================================================="
	@echo " demo is up"
	@echo "   frontend  $(WEB_URL)          (log: $(RUN)/frontend.log)"
	@echo "   backend   $(API_URL)          (log: $(RUN)/backend.log)"
	@echo "   anvil     $(RPC_URL)          (log: $(RUN)/anvil.log)"
	@echo "   registry  $$(cat $(DEPLOYMENT))"
	@echo "   PDFs      ./demo-files/"
	@echo
	@echo " open $(WEB_URL) and drop a file from ./demo-files into the verify screen"
	@echo
	@echo " try it:"
	@echo "   file                                  verdict"
	@echo "   asha-menon-birth-certificate-v2.pdf   VALID"
	@echo "   asha-menon-birth-certificate-v1.pdf   SUPERSEDED (points at v2)"
	@echo "   rahul-iyer-driving-licence.pdf        REVOKED"
	@echo "   rahul-iyer-degree-TAMPERED.pdf        TAMPERED"
	@echo "   never-issued-driving-licence.pdf      NOT_ISSUED"
	@echo
	@echo "   curl -s -F file=@demo-files/asha-menon-birth-certificate-v1.pdf $(API_URL)/verify | jq"
	@echo "   curl -s -F file=@demo-files/rahul-iyer-degree-TAMPERED.pdf $(API_URL)/verify | jq"
	@echo "   curl -s $(API_URL)/verify/BC-2019-004471 | jq       # what a QR scan hits"
	@echo "   curl -s $(API_URL)/credentials/Asha%20Menon | jq    # v1 and v2 side by side"
	@echo
	@echo " stop with: make stop"
	@echo "=============================================================="

## tools: make sure forge and anvil are on PATH
tools:
	@command -v forge >/dev/null || { \
		echo "forge not found. install foundry:"; \
		echo "  curl -L https://foundry.paradigm.xyz | bash && foundryup"; exit 1; }

## test: the contract suite plus the backend's verify state machine
test:
	cd $(CONTRACTS) && forge test -vv
	cd $(BACKEND) && go test ./...

## build: compile contracts and the Go binaries into ./bin
build: test
	@mkdir -p bin
	cd $(BACKEND) && go build -o ../bin/server ./cmd/server && go build -o ../bin/seed ./cmd/seed && go build -o ../bin/profile-seed ./cmd/profile-seed && go build -o ../bin/citizen-seed ./cmd/citizen-seed

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
	@cd $(BACKEND) && PUBLIC_WEB_URL=$(WEB_URL) PUBLIC_API_URL=$(API_URL) setsid --fork ../bin/server < /dev/null > ../$(RUN)/backend.log 2>&1 & echo $$! > $(RUN)/backend.pid
	@echo "waiting for the backend..."
	@for i in $$(seq 1 100); do \
		curl -sf $(API_URL)/health >/dev/null 2>&1 && break; \
		sleep 0.3; \
	done
	@curl -sf $(API_URL)/health >/dev/null || { echo "backend never came up; see $(RUN)/backend.log"; exit 1; }
	@echo "backend up on $(API_URL)"

## profiles: provision demo Supabase users in backend RBAC (requires SEED_*_USER_ID)
profiles:
	@test -n "$(SEED_BIRTH_USER_ID)" || { echo "SEED_BIRTH_USER_ID is required"; exit 1; }
	@test -n "$(SEED_TRANSPORT_USER_ID)" || { echo "SEED_TRANSPORT_USER_ID is required"; exit 1; }
	@test -n "$(SEED_EDUCATION_USER_ID)" || { echo "SEED_EDUCATION_USER_ID is required"; exit 1; }
	./bin/profile-seed -db $(BACKEND)/credentials.db -id "$(SEED_BIRTH_USER_ID)" -email "$(SEED_BIRTH_EMAIL)" -name "Birth Demo Admin" -role ADMIN -department birth
	./bin/profile-seed -db $(BACKEND)/credentials.db -id "$(SEED_TRANSPORT_USER_ID)" -email "$(SEED_TRANSPORT_EMAIL)" -name "Transport Demo Admin" -role ADMIN -department transport
	./bin/profile-seed -db $(BACKEND)/credentials.db -id "$(SEED_EDUCATION_USER_ID)" -email "$(SEED_EDUCATION_EMAIL)" -name "Education Demo Admin" -role ADMIN -department education
	./bin/citizen-seed -db $(BACKEND)/credentials.db -id asha-menon -name "Asha Menon" -email asha.menon@example.test
	./bin/citizen-seed -db $(BACKEND)/credentials.db -id rahul-iyer -name "Rahul Iyer" -email rahul.iyer@example.test

## seed: authenticated demo documents (requires SEED_*_TOKEN)
seed:
	./bin/seed -api $(API_URL) -out demo-files

## frontend: start the Vite dev server in the background
frontend:
	@mkdir -p $(RUN)
	@[ -d $(FRONTEND)/node_modules ] || (cd $(FRONTEND) && npm install)
	@cd $(FRONTEND) && setsid --fork npm run dev < /dev/null > ../$(RUN)/frontend.log 2>&1 & echo $$! > $(RUN)/frontend.pid
	@echo "waiting for the frontend..."
	@for i in $$(seq 1 100); do \
		curl -sf $(WEB_URL) >/dev/null 2>&1 && break; \
		sleep 0.3; \
	done
	@curl -sf $(WEB_URL) >/dev/null || { echo "frontend never came up; see $(RUN)/frontend.log"; exit 1; }
	@echo "frontend up on $(WEB_URL)"

## stop: kill anvil, the backend and the frontend
stop:
	@-[ -f $(RUN)/backend.pid ] && kill $$(cat $(RUN)/backend.pid) 2>/dev/null || true
	@-[ -f $(RUN)/anvil.pid ] && kill $$(cat $(RUN)/anvil.pid) 2>/dev/null || true
	@-[ -f $(RUN)/frontend.pid ] && kill $$(cat $(RUN)/frontend.pid) 2>/dev/null || true
	@-pkill -x server 2>/dev/null || true
	@-pkill -x anvil 2>/dev/null || true
	@-pkill -f "$(FRONTEND)/node_modules/.bin/vite" 2>/dev/null || true
	@-rm -rf $(RUN)
	@echo "stopped"

## clean: throw away all demo state (db, deployed address, generated PDFs)
clean:
	@rm -f $(BACKEND)/credentials.db $(DEPLOYMENT)
	@rm -rf demo-files bin $(CONTRACTS)/broadcast $(CONTRACTS)/cache $(FRONTEND)/dist
	@echo "cleaned"
