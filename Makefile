all:
	docker build -t faryon93/kallax:latest .
push:
	docker push faryon93/kallax:latest
