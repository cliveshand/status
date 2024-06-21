ARG REGISTRY=${CI_REGISTRY}
ARG REPOSITORY=docker.io/golang
ARG TAG=1.21
ARG REPOSITORY2=cgr.dev/chainguard/hugo
ARG TAG2=latest-dev

FROM ${REGISTRY}/${REPOSITORY}:${TAG} as builder

RUN mkdir /hugo
WORKDIR /hugo

# Copy requirements and download packages
COPY go.mod go.sum ./

RUN go mod download

# Copy repo  for compilation
COPY . .

# Compile the app
RUN CGO_ENABLED=0 GOOS=linux go build -a -buildvcs=false -installsuffix cgo -o status .

#FROM ${REGISTRY}/${REPOSITORY2}:${TAG2} as base
FROM ${REPOSITORY2}:${TAG2} as base

# Status Golang Code
COPY --from=builder /hugo/status /hugo/status

WORKDIR /hugo

# HUGO Site Code
COPY archetypes      /hugo/archetypes
COPY content        	/hugo/content
COPY layouts         /hugo/layouts
COPY static		    /hugo/static
COPY Makefile	    /hugo/
COPY hugo.yml	    /hugo/

ENTRYPOINT ["/hugo/status"]
