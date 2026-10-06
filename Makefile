AWS_REGION ?= us-east-1
STACK      ?= ufpa-api
VERSION    ?= v1
PLATFORM   ?= linux/arm64
DESIRED    ?= 2
CERT_ARN   ?=

export AWS_REGION

stack_output = $(shell aws cloudformation describe-stacks --stack-name $(STACK) \
	--query "Stacks[0].Outputs[?OutputKey=='$(1)'].OutputValue" --output text)
REPO_URI = $(call stack_output,RepositoryUri)
ALB_URL  = $(call stack_output,AlbUrl)
CLUSTER  = $(call stack_output,ClusterName)
SERVICE  = $(call stack_output,ServiceName)

.PHONY: test pin build-naive build run-local compare scan-warmup scan \
	bootstrap deploy login push release watch kill-task logs destroy

test:
	go test -cover ./...

pin:
	./scripts/pin-digests.sh Dockerfile

build-naive:
	docker build -f Dockerfile.naive -t ufpa-api:naive .

build:
	docker buildx build --platform $(PLATFORM) --build-arg VERSION=$(VERSION) -t ufpa-api:$(VERSION) --load .

run-local:
	docker run --rm -p 8080:8080 --read-only --cap-drop ALL ufpa-api:$(VERSION)

compare:
	docker images ufpa-api --format 'table {{.Tag}}\t{{.Size}}'

scan-warmup:
	trivy image --download-db-only

scan:
	trivy image --skip-db-update --severity HIGH,CRITICAL ufpa-api:naive
	trivy image --skip-db-update --severity HIGH,CRITICAL ufpa-api:$(VERSION)

bootstrap:
	$(MAKE) deploy DESIRED=0

deploy:
	aws cloudformation deploy --template-file infra/stack.yaml --stack-name $(STACK) \
		--capabilities CAPABILITY_IAM --no-fail-on-empty-changeset \
		--parameter-overrides ImageTag=$(VERSION) DesiredCount=$(DESIRED) \
		$(if $(CERT_ARN),CertificateArn=$(CERT_ARN),)

login:
	aws ecr get-login-password | docker login --username AWS --password-stdin $(firstword $(subst /, ,$(REPO_URI)))

push: login
	docker tag ufpa-api:$(VERSION) $(REPO_URI):$(VERSION)
	docker push $(REPO_URI):$(VERSION)

release: build push deploy

watch:
	@while true; do curl -sf --max-time 2 $(ALB_URL)/v1/info || echo '{"error":"no response"}'; sleep 0.5; done

kill-task:
	aws ecs stop-task --cluster $(CLUSTER) --reason "demo: self-healing" --query 'task.taskArn' --output text \
		--task $$(aws ecs list-tasks --cluster $(CLUSTER) --service-name $(SERVICE) --query 'taskArns[0]' --output text)

logs:
	aws logs tail /ecs/$(STACK) --follow --format short

destroy:
	aws cloudformation delete-stack --stack-name $(STACK)
	aws cloudformation wait stack-delete-complete --stack-name $(STACK)
