FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
ARG SERVICE
RUN CGO_ENABLED=0 go build -o /out/app ./cmd/${SERVICE}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -H appuser
WORKDIR /app
USER appuser
COPY --from=build /out/app /app/app
COPY configs ./configs
ENTRYPOINT ["/app/app"]
