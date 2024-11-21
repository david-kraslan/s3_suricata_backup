FROM golang:1.23.2-alpine AS builder

WORKDIR /app

COPY go.* ./

RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -o bootstrap main.go

FROM public.ecr.aws/lambda/go:1.2024.10.04.19

COPY --from=builder /app/bootstrap ${LAMBDA_TASK_ROOT}/bootstrap

CMD ["./bootstrap"]




