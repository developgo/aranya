workflow "build images" {
  on = "push"
  resolves = [
    "push aranya",
  ]
}

action "login @docker_hub" {
  uses = "actions/docker/login@master"
  secrets = [
    "DOCKER_PASSWORD",
    "DOCKER_USERNAME",
  ]
}

action "build aranya" {
  uses = "actions/docker/cli@master"
  args = "build -t arhatdev/aranya:latest -f docker/aranya.dockerfile ."
}

action "push aranya" {
  needs = ["build aranya", "login @docker_hub"]
  uses = "actions/docker/cli@master"
  args = "push arhatdev/aranya:latest"
}
