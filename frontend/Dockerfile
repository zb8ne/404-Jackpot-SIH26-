# The Vite dev server, containerised. Kept as the dev server rather than a
# static build so the demo machine can still edit a screen and see it reload.
FROM node:22-alpine

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

EXPOSE 5173
# --host binds beyond loopback, which a container must do to be reachable from
# the host at all.
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
