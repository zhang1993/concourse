install:
	go install -v ./cmd/concourse

run:
	CONCOURSE_ADD_LOCAL_USER=test:test,guest:guest \
	CONCOURSE_CLUSTER_NAME=dev \
	CONCOURSE_EXTERNAL_URL=http://localhost:8080 \
	CONCOURSE_ENABLE_LIDAR=true \
	CONCOURSE_ENABLE_GLOBAL_RESOURCES=true \
	CONCOURSE_LOG_LEVEL=debug \
	CONCOURSE_MAIN_TEAM_LOCAL_USER=test \
	CONCOURSE_POSTGRES_DATABASE=concourse \
	CONCOURSE_POSTGRES_HOST=localhost \
	CONCOURSE_POSTGRES_PORT=6543 \
	CONCOURSE_POSTGRES_PASSWORD=dev \
	CONCOURSE_POSTGRES_USER=dev \
		concourse web \
			--kubernetes-worker-kubeconfig=/var/snap/microk8s/current/credentials/client.config \
			--tsa-host-key=./keys/web/tsa_host_key
