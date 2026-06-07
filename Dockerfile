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
COPY briefing ./briefing
COPY email ./email
COPY ingest ./ingest
COPY internal ./internal
COPY cmd ./cmd
COPY --from=email-template /src/email/template/out ./email/template/out
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/gpnews ./cmd/local
RUN CGO_ENABLED=0 GOOS=linux go build -tags lambda.norpc -o /bin/gpnews-lambda ./cmd/lambda

FROM alpine:3.22 AS local
RUN apk add --no-cache ca-certificates
WORKDIR /app
ENV ENVIRONMENT=prod
COPY --from=build /bin/gpnews ./gpnews
RUN mkdir -p cache
CMD ["./gpnews"]

FROM public.ecr.aws/lambda/provided:al2023 AS lambda
WORKDIR /var/task
ENV ENVIRONMENT=prod
ENV CACHE_DIR=/tmp/gpnews-cache
COPY --from=build /bin/gpnews-lambda ./gpnews-lambda
ENTRYPOINT ["./gpnews-lambda"]
