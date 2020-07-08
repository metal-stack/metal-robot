# metal-robot 🤖

A bot helping to automate some tasks on Github and Gitlab.

## Task Descriptions

TBD

## Development

Developing this effectively is a little iffy because you require Github and Gitlab to push their webhooks to your local machine.

When the robot is already deployed into a Kubernetes cluster and webhooks are coming in, you can make use of [telepresence](https://www.telepresence.io/) to proxy the requests to your local machine. Even though it seems a bit scary what telepresence does, it works. The installation of it was wrapped into a Docker container, which is built before start. The container requires privileged and host network to run though. Please take a look at the [Makefile](Makefile) and read about telepresence before you use it. Then decide if you really want to do it.

If you have any better ideas, please open a PR or an issue.

Here is how to do it:

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
# - Ubuntu & Docker
# - KUBECONFIG env var points to the cluster where the robot is deployed
#   - change to the correct context
#   - change to the namespace
#   - the deplyoment needs to be called "metal-robot"
#
# when you are done, exit the container shell
```
