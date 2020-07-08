# metal-robot

A bot helping to automate some tasks on Github and Gitlab. 🤖

## Task Descriptions

TBD

## Development

Developing this effectively is a little iffy because you require Github and Gitlab to push their webhooks to your local machine. When the robot is deployed into a Kubernetes cluster you can make use of [telepresence](https://www.telepresence.io/), which proxies network connections accordingly. Even though I find telepresence quite scary, it works. Please look at the [Makefile](Makefile) before you use it and decide if you really want to do it.

```
# build the binary
make

# start the robot
bin/metal-robot \
  --bind-addr 0.0.0.0 \
  --github-webhook-secret <something> \
  --gitlab-webhook-secret <something>

# in another terminal window run
make local
# this requires:
# - Ubuntu
# - kubectl installed
# - KUBECONFIG env var points to the robot cluster

# when you are done, exit the container shell
```

If you have any better ideas, let me know.
