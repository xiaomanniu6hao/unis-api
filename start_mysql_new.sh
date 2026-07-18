docker run -d \
  --name mysql-unisapi \
  -p 3307:3306 \
  -v $(pwd)/data/mysql-new:/var/lib/mysql \
  -e TZ=Asia/Shanghai \
  -e MYSQL_ROOT_PASSWORD=123456 \
  -e MYSQL_USER=unisapi \
  -e MYSQL_PASSWORD=123456 \
  -e MYSQL_DATABASE=unisapi \
  --restart always \
  mysql:8.0
