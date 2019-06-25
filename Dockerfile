# NOTE: this Dockerfile is purely for local development! it is *not* used for
# the official 'concourse/concourse' image.

FROM concourse/concourse:local

# download go modules separately so this doesn't re-run on every change
WORKDIR /src
COPY go.mod .
COPY go.sum .
COPY kube-config /root/.kube/config
RUN grep '^replace' go.mod || go mod download

# build Concourse without using 'packr' and set up a volume so the web assets
# live-update
COPY . .
RUN go build -gcflags=all="-N -l" -o /usr/local/concourse/bin/concourse \
      ./cmd/concourse
VOLUME /src

