all: run


keys:
	mkdir -p keys
	concourse generate-key -t rsa -b 1024 -f ./keys/session_signing_key
	concourse generate-key -t ssh -b 1024 -f ./keys/tsa_host_key
	concourse generate-key -t ssh -b 1024 -f ./keys/worker_key
	cp ./keys/worker_key.pub ./keys/authorized_worker_keys
.PHONY: keys


run: install
	CONCOURSE_ADD_LOCAL_USER=test:test,guest:guest \
	CONCOURSE_CLUSTER_NAME=dev \
	CONCOURSE_EXTERNAL_URL=http://localhost:8080 \
	CONCOURSE_MAIN_TEAM_LOCAL_USER=test \
	CONCOURSE_POSTGRES_DATABASE=concourse \
	CONCOURSE_POSTGRES_HOST=localhost \
	CONCOURSE_POSTGRES_PASSWORD=dev \
	CONCOURSE_POSTGRES_USER=dev \
	CONCOURSE_SESSION_SIGNING_KEY=./keys/session_signing_key \
	CONCOURSE_TSA_AUTHORIZED_KEYS=./keys/authorized_worker_keys \
	CONCOURSE_TSA_HOST_KEY=./keys/tsa_host_key \
		concourse web


install:
	go install -v ./cmd/concourse



database:
	docker run \
		--detach \
		--publish 5432:5432 \
		--env POSTGRES_DB=concourse \
		--env POSTGRES_USER=dev \
		--env POSTGRES_PASSWORD=dev \
		postgres 


worker:
	docker run \
		--detach \
		--privileged \
		--volume $(shell pwd)/keys:/concourse-keys \
		--env CONCOURSE_LOG_LEVEL=debug \
		--env CONCOURSE_TSA_HOST=host.docker.internal:2222 \
		--env CONCOURSE_BAGGAGECLAIM_DRIVER=overlay \
		--stop-signal SIGUSR2 \
		concourse/concourse:local \
		worker
.PHONY: worker
