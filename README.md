# k8s-controller-ingress

## Overview
The `k8s-controller-ingress` project is a Kubernetes controller that manages ingress resources based on deployment events. It watches for changes in Kubernetes deployments and automatically creates, updates, or deletes corresponding services and ingress resources.

## Project Structure
```
k8s-controller-ingress
├── cmd
│   └── main.go          # Entry point of the application
├── internal
│   ├── controller
│   │   └── deployWatcher.go  # Handles deployment events
│   └── configs
│       └── config.go        # Configuration management
├── go.mod                    # Go module definition
├── go.sum                    # Dependency checksums
└── README.md                 # Project documentation
```

## Setup Instructions
1. **Clone the repository:**
   ```bash
   git clone https://github.com/Digode/k8s-controller-ingress.git
   cd k8s-controller-ingress
   ```

2. **Install dependencies:**
   Ensure you have Go installed, then run:
   ```bash
   go mod tidy
   ```

3. **Set up environment variables:**
   Configure the necessary environment variables as defined in `internal/configs/config.go`.

## Usage
To run the controller, execute the following command:
```bash
go run cmd/main.go
```

## Usage Sample
1. ***Install Ingress Controller***
   ```bash
   helm upgrade --install public-ingress-nginx ingress-nginx/ingress-nginx -n public-ingress-nginx -f ./setup/values-ingress.yaml --create-namespace
   ```

   Verify all working
   ```bash
   kubectl get ingressclass
   kubectl get pod --watch -n public-ingress-nginx
   ```

2. ***Install Deploy Watcher***
   ```bash
   kubectl apply -f ./setup/k8sdeploy.yaml
   ```

   Verify all working
   ```bash
   kubectl get serviceaccount,clusterrole.rbac.authorization.k8s.io,clusterrolebinding.rbac.authorization.k8s.io,deploy -n deploy-watcher | grep deploy-watcher
   kubectl get pod --watch -n public-ingress-nginx
   ```

   Checking logs of deploy-watcher
   ```bash
   kubectl logs -f -l app=deploy-watcher -n deploy-watcher
   ```
4. ***Create namespace***
   ```bash
   kubectl create ns sample
   ```

5. ***Start tunnel of minikube***
   ```bash
   minikube tunnel
   ```
3. ***Add deploy + service + ingress***
   ```
   kubectl apply -f ./sample/full.yaml -n sample

   # If need, add address to hosts: sudo sh -c 'echo "127.0.0.1 nginx-not.localhost" > /etc/hosts'
   ```
   
   Verify resources
   ```bash
   kubectl get deploy,service,ingress,pod -n sample
   ```

   Test the host
   ```bash
   curl -v nginx-not.localhost
   ```

4. ***Add deploy with watcher***
   ```
   kubectl apply -f ./sample/with-watcher.yaml -n sample
   # If need, add address to hosts: sudo sh -c 'echo "127.0.0.1 nginx.localhost" > /etc/hosts'
   ```
   
   Verify resources
   ```bash
   kubectl get deploy,service,ingress,pod -n sample
   ```

   Test the host
   ```bash
   curl -v nginx.localhost
   ```

5. ***Clean***
   ```bash
   kubectl delete -f ./sample/with-watcher.yaml
   kubectl delete -f ./sample/full.yaml
   kubectl delete -f ./setup/k8sdeploy.yaml
   helm uninstall -n public-ingress-nginx public-ingress-nginx
   kubectl delete ns sample
   ```

## Contributing
Contributions are welcome! Please submit a pull request or open an issue for any enhancements or bug fixes.

## License
This project is licensed under the MIT License. See the LICENSE file for details.