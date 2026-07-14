docker run -d \
  --name mysql \
  -p 3306:3306 \
  -v $(pwd)/data/mysql:/var/lib/mysql \
  -e TZ=Asia/Shanghai \
  -e MYSQL_ROOT_PASSWORD=123456 \
  -e MYSQL_USER=mixapi \
  -e MYSQL_PASSWORD=123456 \
  -e MYSQL_DATABASE=mix-api \
  --restart always \
  mysql:8.0
