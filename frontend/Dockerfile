FROM node:24.13.1-alpine AS builder
ARG PUBLIC_APP_VERSION
ENV PUBLIC_APP_VERSION=$PUBLIC_APP_VERSION

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM node:24.13.1-alpine

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder --chown=app:app /app/dist ./dist
COPY --from=builder --chown=app:app /app/node_modules ./node_modules
COPY --from=builder --chown=app:app /app/package.json ./

USER app

EXPOSE 4321

CMD ["node", "./dist/server/entry.mjs"]
