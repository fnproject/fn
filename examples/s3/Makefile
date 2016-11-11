IMAGE=iron/lambda-node-aws-example

create: Dockerfile
	docker build -t $(IMAGE) .

test:
	docker run --rm -it -e PAYLOAD_FILE=/mnt/example-payload.json -e AWS_ACCESS_KEY_ID=change-here -e AWS_SECRET_ACCESS_KEY=change-here -v `pwd`:/mnt $(IMAGE)
