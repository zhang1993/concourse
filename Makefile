build:
	go install -v ./cmd/concourse

image:
	DOCKER_BUILDKIT=1 \
		docker build -t concourse/concourse:local .

up: image
	docker-compose up -d

down:
	docker-compose down
	docker volume prune -f

workers-table:
	docker exec concourse_db_1 psql \
		--dbname=concourse \
		-U dev \
		-c 'select name,zone,baggageclaim_peer_url from workers;'

pipeline:
	fly -t local login -u test -p test
	fly -t local set-pipeline -n -p test -c ./pipeline.yml
	fly -t local unpause-pipeline -p test
