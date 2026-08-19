# A thin wrapper so Railway's Docker-image services (which have no start
# command of their own) run Anvil with the flags the rest of the stack
# expects, matching docker-compose.yml's local equivalent.
FROM ghcr.io/foundry-rs/foundry:latest
ENTRYPOINT ["anvil", "--host", "::", "--silent"]
EXPOSE 8545
