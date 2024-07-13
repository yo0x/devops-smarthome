 23  helm repo update
   24  helm install -n awx --create-namespace my-awx-operator awx-operator/awx-operator
   25  helm install my-awx-operator awx-operator/awx-operator -n awx --create-namespace -f myvalues.yaml\n
   26  helm install my-awx-operator awx-operator/awx-operator -n awx --create-namespace
   27  helm uninstall my-awx-operator -n awx
   28  cd Documents
   29  cd devops-smarthome
   30  ls
   31  cd awx
   32  cd ..
   33  kubectl apply -k awx/kustomization.yaml
   34  kubectl apply -k awx
   35  kubectl get pods -n awx
   36  $ kubectl config set-context --current --namespace=awx\n
   37  $ kubectl config set-context --current --namespace\n
   38  kubectl config set-context --current --namespace=awx
   39  kubectl apply -k awx
   40  $ kubectl logs -f deployments/awx-operator-controller-manager -c awx-manager\n
   41  kubectl logs -f deployments/awx-operator-controller-manager -c awx-manager