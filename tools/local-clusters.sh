#!/usr/bin/env bash

set -euo pipefail

KUBERNETES_VERSION="${KUBERNETES_VERSION:-1.37.0}"

echo "Setup local clusters using Kubernetes $KUBERNETES_VERSION"

create_cluster_if_missing() {
    local name="$1"

    if kind get clusters 2>/dev/null | grep -qx "$name"; then
        echo -e "Cluster '${name}' already exists, skipping creation"
    else
        echo -e "Cluster '${name}' does not exist, creating..."
        kind create cluster --name "$name" --image kindest/node:v$KUBERNETES_VERSION
    fi
}

echo -e "Create clusters: tools, dev, prod"

create_cluster_if_missing tools
create_cluster_if_missing dev
create_cluster_if_missing prod

echo -e "Setup tools cluster"

helm upgrade --install cert-manager \
    oci://quay.io/jetstack/charts/cert-manager:v1.21.2 \
    --namespace cert-manager \
    --create-namespace \
    --values infrastructure/kubernetes/_bootstrap/certmanager.values.yaml \
    --kube-context kind-tools

helm upgrade --install kargo \
    oci://ghcr.io/akuity/kargo-charts/kargo:1.11.4 \
    --namespace kargo \
    --create-namespace \
    --values infrastructure/kubernetes/_bootstrap/kargo.values.yaml \
    --kube-context kind-tools

helm upgrade --install argocd \
    oci://ghcr.io/argoproj/argo-helm/argo-cd:10.9.2 \
    --namespace argocd \
    --create-namespace \
    --values infrastructure/kubernetes/_bootstrap/argocd.values.yaml \
    --kube-context kind-tools

echo -e "Register dev/prod clusters in argocd"

echo -e "Waiting "
kubectl wait --for=condition=available --timeout=120s \
    deployment/argocd-server deployment/argocd-repo-server \
    -n argocd --context kind-tools

kubectl config set-context kind-tools --namespace=argocd
argocd login kind-tools --core --kube-context kind-tools

register_cluster() {
    local env="$1"
    local context="kind-${env}"
    local internal_server="https://${env}-control-plane:6443"

    local secret
    secret=$(kubectl --context kind-tools -n argocd get secrets \
        -l "argocd.argoproj.io/secret-type=cluster,env=${env}" \
        -o jsonpath="{.items[0].metadata.name}" 2>/dev/null || true)

    if [ -n "$secret" ]; then
        echo -e "Cluster '${env}' already registered in argocd, skipping"
        return
    fi

    argocd cluster add "$context" \
        --name "$env" \
        --label "env=${env}" \
        --kube-context kind-tools \
        --yes

    secret=$(kubectl --context kind-tools -n argocd get secrets \
        -l "argocd.argoproj.io/secret-type=cluster,env=${env}" \
        -o jsonpath="{.items[0].metadata.name}")

    kubectl --context kind-tools -n argocd patch secret "$secret" \
        --type merge \
        -p "{\"data\":{\"server\":\"$(echo -n "$internal_server" | base64 -w0)\"}}"
}

register_cluster dev
register_cluster prod
