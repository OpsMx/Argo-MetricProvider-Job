FROM golang:1.19-alpine AS build

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 go build -o /Argo-MetricProvider-Job

##
## Deploy
##

FROM gcr.io/distroless/base-debian10:debug

WORKDIR /
COPY --from=build /Argo-MetricProvider-Job /Argo-MetricProvider-Job

ENTRYPOINT ["/Argo-MetricProvider-Job"]