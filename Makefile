OUTPUT_DIR=bin
OUTPUT_PATH=${OUTPUT_DIR}/${BINARY_NAME}

.PHONY: deploy
## deploy: deploy the application
deploy:
	terraform init
	terraform apply

.PHONY: destroy
## destroy: clean up the deployed infrastructure
destroy:
	terraform destroy

.PHONY: build
## build: build the application
build: clean
	go build -o ${OUTPUT_PATH}

