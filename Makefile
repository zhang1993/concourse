KUBECONFIG ?= "/var/snap/microk8s/current/credentials/client.config"


install:
	go install -v ./cmd/concourse

run: install
	CONCOURSE_ADD_LOCAL_USER=test:test,guest:guest \
	CONCOURSE_ENABLE_GLOBAL_RESOURCES=true \
	CONCOURSE_EXTERNAL_URL=http://localhost:8080 \
	CONCOURSE_LIDAR_SCANNER_INTERVAL=10s \
	CONCOURSE_MAIN_TEAM_LOCAL_USER=test \
	CONCOURSE_POSTGRES_DATABASE=concourse \
	CONCOURSE_POSTGRES_HOST=localhost \
	CONCOURSE_POSTGRES_PASSWORD=dev \
	CONCOURSE_POSTGRES_PORT=6543 \
	CONCOURSE_POSTGRES_USER=dev \
		concourse web \
			--kubernetes-worker-kubeconfig=$(KUBECONFIG) \
			--tsa-host-key=./keys/web/tsa_host_key

debug:
	CONCOURSE_ADD_LOCAL_USER=test:test,guest:guest \
	CONCOURSE_ENABLE_GLOBAL_RESOURCES=true \
	CONCOURSE_EXTERNAL_URL=http://localhost:8080 \
	CONCOURSE_LIDAR_SCANNER_INTERVAL=10s \
	CONCOURSE_MAIN_TEAM_LOCAL_USER=test \
	CONCOURSE_POSTGRES_DATABASE=concourse \
	CONCOURSE_POSTGRES_HOST=localhost \
	CONCOURSE_POSTGRES_PASSWORD=dev \
	CONCOURSE_POSTGRES_PORT=6543 \
	CONCOURSE_POSTGRES_USER=dev \
		dlv debug ./cmd/concourse -- web \
			--kubernetes-worker-kubeconfig=$(KUBECONFIG) \
			--tsa-host-key=./keys/web/tsa_host_key

db:
	docker-compose up -d db
