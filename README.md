# bot-k8s

A small Kubernetes demo written in Go: a **bot** watches Pods and pushes events to Redis; a **worker** consumes the queue and creates Batch **Jobs** plus **LoadBalancer** Services in the cluster. It is useful for experimenting with in-cluster clients, Redis-backed work queues, and RBAC.

## Architecture

| Component | Role |
|-----------|------|
| **Redis** | Holds list `job-queue` (pending) and `job-processing` (in-flight during `BRPOPLPUSH`). |
| **Bot** | Lists Pods in the `default` namespace every 15s, `LPUSH`es JSON events (`namespace`, `podName`, `jobId` = pod UID), and patches pods with a sample annotation (Cilium-style placeholder). |
| **Worker** | Blocks on `BRPOPLPUSH` from `job-queue` → `job-processing`, then creates a `Job` named `mock-<jobId>` and a matching `Service` (type `LoadBalancer`). On success it removes the payload from `job-processing`; bad JSON is acknowledged by removing the poison message. |

Both apps use the Kubernetes **in-cluster** config and connect to Redis at `redis:6379` (the Helm chart wires the `redis` Service DNS name).

## Repository layout

- `bot/` — bot source and `Dockerfile`
- `worker/` — worker source and `Dockerfile`
- `charts/` — Helm chart `bot-system` (Redis, bot Deployment, worker Deployment, ServiceAccount, ClusterRole/Binding)
- `k8s/` — example raw manifests (alternative to Helm)

## Prerequisites

- A Kubernetes cluster
- [Helm](https://helm.sh/) 3.x (recommended)
- [Docker](https://docs.docker.com/get-docker/) with **BuildKit** enabled (the Dockerfiles use `RUN --mount=type=cache` for faster builds)
- Go 1.25+ if you run or test the programs locally (modules live under `bot/` and `worker/` separately)

## Build container images

From each service directory, build and tag images as needed. Example:

```bash
cd bot
docker build -t your-registry/bot:tag .

cd ../worker
docker build -t your-registry/worker:tag .
```

Push the images to a registry your cluster can pull from, then set `images.bot` and `images.worker` in `charts/values.yaml` (or override with `--set` when installing).

## Deploy with Helm

```bash
helm install bot-system ./charts -n default --create-namespace
```

Override namespace and images if you use a non-default setup:

```bash
helm install bot-system ./charts \
  --namespace my-ns --create-namespace \
  --set targetNamespace=my-ns \
  --set images.bot.repository=your-registry/bot \
  --set images.bot.tag=latest \
  --set images.worker.repository=your-registry/worker \
  --set images.worker.tag=latest
```

**RBAC:** The chart binds a ClusterRole so the bot and worker can list/patch Pods, and create/list Jobs and Services. Adjust `charts/templates/rbac.yaml` if you want narrower permissions.

## Raw manifests

The `k8s/` directory contains standalone YAML you can apply with `kubectl` if you prefer not to use Helm. Keep RBAC and image references aligned with your environment.

## Configuration reference

Main knobs are in `charts/values.yaml`:

- `targetNamespace` — namespace for workloads and ServiceAccount subject
- `replicaCount` — replicas for bot, worker, and Redis
- `images.*` — container repositories, tags, pull policies
- `serviceAccount.name` — name used by Deployments and RoleBinding
- `redisService` — Redis Service name and port (must match what the Go code expects: host `redis`, port `6379`)

## Implementation notes

- The bot and worker hardcode the **default** namespace for Kubernetes API calls (`Pods`, `Jobs`, `Services`). If you deploy to another namespace, update the Go code or keep workloads in `default` for the demo to behave as written.
- LoadBalancer Services require a cluster that can provision them (e.g. cloud provider or a local implementation like MetalLB).
- This project is intended as a **demo / learning** scratchpad, not production-hardened (minimal error handling, broad RBAC for simplicity).

## License

If you add a license, document it here.
