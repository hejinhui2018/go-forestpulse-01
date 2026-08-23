FROM golang:1.23.12 AS build
WORKDIR /src
ENV GOTOOLCHAIN=local CGO_ENABLED=0
COPY . .
RUN go test ./... -count=1
RUN go vet ./...
RUN go build -trimpath -o /out/forestpulse ./cmd/forestpulse-server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/forestpulse /usr/local/bin/forestpulse
VOLUME ["/var/lib/forestpulse"]
ENV FORESTPULSE_DATA_PATH=/var/lib/forestpulse/state.json
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/forestpulse"]

