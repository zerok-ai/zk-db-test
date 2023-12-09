scriptDir=$(dirname -- "$(readlink -f -- "$BASH_SOURCE")")

helpFunction()
{
   echo $1
   echo "
        Usage:
        $0
        -P CLUSTER_PROVIDER  [gke|minikube|eks]
        -D                   Dry run
        -h                   to see this help text
        "
   echo ""
   exit 1 # Exit script after printing help
}

while getopts "cdDhP:" opt
do
   case "$opt" in
      P ) CLUSTER_PROVIDER="$OPTARG" ;;
      D )
            # declare the variable for all the subscripts
        	declare -x DRY_RUN=1
         declare -x HELM_DRY_RUN="--dry-run"
			declare -x KUBERNETES_DRY_RUN="--dry-run=client"
        ;;
      h ) helpFunction "$opt" ;;
      ? ) helpFunction "$opt" ;;
   esac
done

# check for the mandatory value of cluster provider
#if [[ -z "$CLUSTER_PROVIDER" || ( "$CLUSTER_PROVIDER" != "gke"  &&  "$CLUSTER_PROVIDER" != "aws"  &&  "$CLUSTER_PROVIDER" != "minikube") ]]
#then
#   helpFunction "[Error] Value for cluster provider not found"
#   exit 1;
#fi

# add helm repo for prometheus
echo '---------------------- Updating helm repo for kube-prometheus-stack'
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# create namespace for monitoring stack
echo '---------------------- Creating namespace - `monitoring`'
kubectl create namespace monitoring $KUBERNETES_DRY_RUN

# install kube-prometheus
echo '✨ ------------------- Installing kube-prometheus-stack'
helm upgrade --install prom prometheus-community/kube-prometheus-stack \
	--namespace monitoring \
	--values $scriptDir/values/prometheus-grafana.yaml $HELM_DRY_RUN

echo '✨ --------------------- Installing monitoring ingress'
kubectl apply -f $scriptDir/monitoring-ingress.yaml $KUBERNETES_DRY_RUN

echo '✨ --------------------- Installing redis'
helm upgrade zk-redis zk-redis/zk-redis --install --create-namespace --namespace zk-client --version 0.1.0-alpha --wait -f ${scriptDir}/values/redis-cluster-values.yaml
