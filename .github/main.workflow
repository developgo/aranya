workflow "build images" {
  on = "push"
  resolves = [
    "push aranya", "push arhat-docker-grpc"
  ]
}

action "login@dockerhub" {
  uses = "actions/docker/login@master"
  secrets = [
    "DOCKER_PASSWORD",
    "DOCKER_USERNAME",
  ]
}

action "build aranya" {
  uses = "actions/docker/cli@master"
  args = "build -t arhatdev/aranya:latest --build-arg TARGET=aranya -f cicd/docker/app.dockerfile ."
}

action "build arhat-docker-grpc" {
  uses = "actions/docker/cli@master"
  args = "build -t arhatdev/arhat-docker-grpc:latest --build-arg TARGET=arhat-docker-grpc -f cicd/docker/app.dockerfile ."
}

action "push aranya" {
  needs = ["build aranya", "login@dockerhub"]
  uses = "actions/docker/cli@master"
  args = "push arhatdev/aranya:latest"
}

action "push arhat-docker-grpc" {
  needs = ["build arhat-docker-grpc", "login@dockerhub"]
  uses = "actions/docker/cli@master"
  args = "push arhatdev/arhat-docker-grpc:latest"
}
