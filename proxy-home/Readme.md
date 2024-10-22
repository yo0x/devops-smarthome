.:53 {
    errors
    health
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
    }
    hosts {
       192.168.1.10 myservice.example.com
       fallthrough
    }
    prometheus :9153
    forward . /etc/resolv.conf
    cache 30
    loop
    reload
    loadbalance
}
---
kubectl edit configmap coredns -n kube-system

kubectl rollout restart deployment coredns -n kube-system

kubectl create secret generic cloudflare-api-token-secret --from-literal=apiKey=<your-cloudflare-api-token> -n cert-manager