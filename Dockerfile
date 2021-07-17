FROM golang:alpine as build

RUN apk add git

WORKDIR /app
COPY go.mod go.sum /app/
RUN go mod download
COPY . .
RUN go build .

FROM alpine:3
RUN apk --no-cache add ca-certificates tzdata
COPY --from=build /app/opsgenie-reporter /bin/
ENTRYPOINT ["/bin/opsgenie-reporter"]
