# Shopping list API

## Prerequisites
- Go 1.24.5
- Terraform
- AWS CLI
- GNU make(optional)

## OpenAPI spec

The project contains an OpenAPI spec that was used to generate boilerplate code for handling requests/responses. The spec file can be found here:
[OpenAPI Spec](/api.yaml)

## AWS Infrastructure

Key:
- Security group that is used to restrict database access to only the Lambda function
- RDS PostgreSQL instance in private subnet
- DB credentials and API key injected as environment variables into the lambda
- API Gateway in proxy mode for integration with the lambda function. This will pass all http traffic that it receives to the lambda
- Single executable deployment(along with database migrations)

## Deployment steps

To deploy the application, you will need to first setup your AWS credentials. If you have this setup already, you can skip this step.
You will need to first generate them from the AWS dashboard and provide them to the next command:

```shell
aws configure
```

To deploy the infrastructure, run:
```sh
make deploy
```

After running this, you will be asked to provide credentials for the PostgreSQL database and also a static API key.

## Tearing down the infrastructure

```shell
make destroy
```

## API usage
Get the invoke url from the AWS dashboard for the api gateway that was created with name: "shopping-list-api-dev-api"

1. Add shopping list request: 
```bash
 curl -X POST --location "${URL}/shopping-lists" \
    -H "X-Api-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{
          "name": "ShoppingListName"
        }'
```

Example response:
```json
{
    "createdAt": "0001-01-01T00:00:00Z",
    "id": 1,
    "name": "ShoppingListName",
    "shoppingItems": [],
    "version": 1
}
```

2. Get all shoping lists request:
```shell
curl -X GET --location "${URL}/shopping-lists" -H "X-Api-Key: ${API_KEY}"
```
Example response:
```json
[
    {
        "createdAt": "0001-01-01T00:00:00Z",
        "id": 1,
        "name": "ShoppingListName",
        "shoppingItems": [],
        "version": 1
    }
]
```

3. Delete shopping list:
```shell
curl -X DELETE --location "${URL}/shopping-lists/1" -H "X-Api-Key: ${API_KEY}"
```

4. Add item to shopping list request:
```shell
curl -X POST --location "${URL}/shopping-lists/1/items" \
    -H "X-Api-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{
          "name": "Shopping list item 1",
          "quantity": 50
        }'
```

Example response:
```json
{
    "completed": false,
    "id": 1,
    "name": "Shopping list item 1",
    "quantity": 50,
    "version": 1
}
```

5. Update shopping list item request:
```shell
curl -X PUT --location "${URL}/shopping-lists/1/items/1" \
    -H "X-Api-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{
          "name": "shopping item 1",
          "quantity": 2,
          "version": 1
        }'

```

Example response:
```json
{
    "completed": false,
    "id": 1,
    "name": "shopping item 1",
    "quantity": 2,
    "version": 2
}
```

6. Tick shopping item request:
```shell
curl -X PATCH --location "${URL}/shopping-lists/1/items/1/toggle" \
    -H "X-Api-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{
          "version": 2
        }'
```

Example response:
```json
{
    "completed": true,
    "id": 1,
    "name": "shopping item 1",
    "quantity": 2,
    "version": 3
}
```

If the client tries to update or tick a shopping item when the client's local state is stale, he will receive a ```HTTP 409``` status. This is done using optimistic locking and the ```version``` field that is returned from all API endpoints. If the version field doesn't match what is currently in the database, the request will fail.

## Design decisions

1. Use a spec-first approach instead of a code-first approach for OpenAPI. In this case, spec-first was chosen because it allowed the code generation to handle all the boilerplate code when handling HTTP requests. I prefer this choice most of the time, but one of its major drawbacks is that in some cases it lacks the flexibility of an entire API written by hand. Here you are constrained by the capabilities of the generator and may need to do some workarounds in some cases.

2. For ease of use, I chose to do the database migrations at an application level. The drawback for this is the increased bundle size for the lambda along with the increased overhead when a cold-start happens.

3. API Gateway is running in proxy mode, which will just redirect all the requests to the lambda. This allows us to skip the request/response configuration part of the non-proxy variant.

4. Used the ```github.com/awslabs/aws-lambda-go-api-proxy``` library to handle the API Gateway events and easily integrate them with the usual flow of a REST app written in Go. This allows us to easily configure our code to run both on Lambda and in a non-serverless environment.