# metal-robot 🤖

A bot helping to automate some tasks on Github and Gitlab.

## Task Descriptions

- Automatically add releases of certain repositories to the [releases](https://github.com/metal-stack/releases) repository (`develop` branch)
- Automatically add commit and PR for swagger client repositories to update their clients (`auto-generate/<version>` branch, Github action needs to generate the client and push it back to this branch)
- Adds a comment on [docs](https://github.com/metal-stack/docs) PRs to a rendered preview

## Development

Developing this effectively is a little iffy because you require Github and Gitlab to push their webhooks to your local machine.

When the robot is already deployed into a Kubernetes cluster and webhooks are coming in, you can make use of [telepresence](https://www.telepresence.io/) to proxy the requests to your local machine. Even though it seems a bit scary what telepresence does, it works. The installation of it was wrapped into a Docker container, which is built before start. The container requires privileged and host network to run though. Please take a look at the [Makefile](Makefile) and read about telepresence before you use it. Then decide if you really want to do it.

If you have any better ideas, please open a PR or an issue.

Here is how to do it:

```
# build and start the robot locally
make start

# in another terminal window run
make swap
# this requires:
# - Ubuntu & Docker
# - KUBECONFIG env var points to the cluster where the robot is deployed
# - the deplyoment in the cluster needs to be called "metal-robot"
#
# when you are done, exit the container shell
# if your session was really long, telepresence sometimes does not swap
# back the deployment properly. delete the temporary deployment and redeploy
# the robot if necessary.
```
