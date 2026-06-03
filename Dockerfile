FROM node:24-alpine AS email-template
WORKDIR /src/email/template
RUN corepack enable
COPY email/template/package.json email/template/pnpm-lock.yaml email/template/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY email/template/ ./
RUN pnpm typecheck && pnpm export

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY briefing ./briefing
COPY ingest ./ingest
COPY --from=email-template /src/email/template/out ./email/template/out
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/gpnews .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
WORKDIR /app
ENV ENVIRONMENT=prod
COPY --from=build /bin/gpnews ./gpnews
RUN mkdir -p cache
CMD ["./gpnews"]
