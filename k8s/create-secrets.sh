namespace=green

kubectl create secret tls schema-registry-proxy-tls -n ${namespace} \
  --cert=tls.crt \
  --key=tls.key

kubectl create secret generic schema-registry-proxy-ca -n ${namespace} \
  --from-file=ca.crt=ca.crt