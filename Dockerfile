#Stage 1: Build
FROM golang:1.23.2-alpine AS builder

WORKDIR /app

COPY go.* ./

RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -o bootstrap main.go

#Stage 2: Lambda runtime image
FROM public.ecr.aws/lambda/go:1 

COPY --from=builder /app/bootstrap ${LAMBDA_TASK_ROOT}/bootstrap

CMD ["./bootstrap"]




