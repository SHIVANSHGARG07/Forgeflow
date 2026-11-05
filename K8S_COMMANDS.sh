# K8S_COMMANDS.sh
# Saari Kubernetes related commands yahan likhte rahenge, jaise jaise seekhte jayenge.
# Isko copy-paste karke chalana hai, seedha execute nahi karna (isliye .sh hone ke
# bawajood ye "run" nahi karna, bas reference ke liye hai, SETUP_COMMANDS.sh jaisa hi).


# ---------------------------------------------------------------------------
# STEP 1: kind cluster banana (Kubernetes IN Docker)
# Kya karta hai: laptop pe ek Docker container ke andar poora K8s cluster
# (control-plane + worker, dono simulate) chala deta hai.
# ---------------------------------------------------------------------------
kind create cluster --name forgeflow


# ---------------------------------------------------------------------------
# STEP 2 (CHECK): Cluster verify karna
# ---------------------------------------------------------------------------
kubectl cluster-info --context kind-forgeflow
kubectl get nodes


# ---------------------------------------------------------------------------
# STEP 3: Local Docker images ko kind cluster ke andar load karna
# Kya karta hai: tumhari local build ki hui images (docker build se bani)
# ko seedha cluster ke andar copy karta hai, bina Docker Hub ke through gaye.
# Postgres ke liye zarurat nahi - wo public image hai, K8s khud pull kar lega.
# ---------------------------------------------------------------------------
kind load docker-image jobflow-scheduler:test --name forgeflow
kind load docker-image jobflow-worker:test --name forgeflow


# ---------------------------------------------------------------------------
# STEP 4 (CHECK): Images load hui ya nahi, cluster ke andar jaake dekhna
# ---------------------------------------------------------------------------
docker exec -it forgeflow-control-plane crictl images
