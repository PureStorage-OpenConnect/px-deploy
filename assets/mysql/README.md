## run a mysql application consistent snapshot demo

#### 0. have px cluster up & running

#### 1. apply mysql namespace & application

```
kubectl apply -f /assets/mysql/mysql.yml
```

#### 2. check for mysql pod readiness

```
kubectl wait --for=condition=ready pod -l app=mysql -n mysql
```

#### 3. apply demo data

```
POD=$(kubectl get pods -n mysql -l app=mysql -ojson | jq -r '.items[0].metadata.name')
```

```
kubectl exec $POD -n mysql -it -- /bin/mysql -u mysql -psupermysql < /assets/mysql/sample-data.sql
```

#### 4. check for data within DB

```
kubectl exec $POD -n mysql -it -- /bin/mysql -u mysql -psupermysql pxdemo -e 'SELECT * FROM users'
```

#### 5. apply pre/post rules

```
kubectl apply -f /assets/mysql/px-mysql-pre-rule.yml

kubectl apply -f /assets/mysql/px-mysql-post-rule.yml
```

#### 6. run snapshot

```
kubectl apply -f /assets/mysql/snapshot.yml
```

#### 7. validate snapshot

```
kubectl get volumesnapshot.volumesnapshot.external-storage.k8s.io/mysql-snap -n mysql -oyaml
```

(optionally follow stork pod logs)

#### 8. delete table

```
kubectl exec $POD -n mysql -it -- /bin/mysql -u mysql -psupermysql pxdemo -e 'DROP TABLE users; SHOW TABLES'
```

#### 9. run snapshot restore

```
kubectl apply -f /assets/mysql/restore-snap.yml
```

check status

```
kubectl get volumesnapshotrestore.stork.libopenstorage.org/mysql-snap-inrestore -n mysql -o yaml
```

(wait for status successful)

#### 10. validate data restore

check for mysql pod readiness

```
kubectl wait --for=condition=ready pod -l app=mysql -n mysql
```

update pod name variable

```
POD=$(kubectl get pods -n mysql -l app=mysql -ojson | jq -r '.items[0].metadata.name')
```

```
kubectl exec $POD -n mysql -it -- /bin/mysql -u mysql -psupermysql pxdemo -e 'SELECT * FROM users'
```