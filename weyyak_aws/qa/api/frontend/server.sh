export SERVICE_PORT=3003 
export DB_SERVER=msapiqa-rds.z5.com 
export DB_SERVER_READER=msapiqa-rds-ro.z5.com 
export DB_PORT=5432 
export DB_USER=weyyak_aurora 
export DB_PASSWORD=M5Ltay9sDY93khvmcpNE 
export DB_DATABASE=wk_frontend 
export CONTENT_DB_DATABASE=wyk_content 
export FRONTEND_CONFIG_DB_DATABASE=wyk_frontend_config 
export USER_DB_DATABASE=wk_user_management 
export DEFAULT_PAGE_SIZE=20 
export CONTENT_COMMON_API_URL= 
export CMS=https://apiqa.wyk.z5.com/v1/ 
export UM=https://apiqa.wyk.z5.com/v1/ 
export IMAGES=https://contents-uat.weyyak.z5.com/ 
export GEO_LOCATION=https://geo.weyyak1.z5.com 
export VIDEO_API=https://api-weyyak.akamaized.net/get_info/ 
export AD_TAG_URL=https://s3.ap-south-1.amazonaws.com/z5xml/mobile_apps_ads_ios.xml 
export BASE_URL=https://ynk2yz6oak.execute-api.ap-south-1.amazonaws.com/weyyak-fo-ms-api-qa/ 
export USER_ACTIVITY_URL=https://msapiqa-events.z5.com/event/activity 
export CONTENT_TYPE_URL=https://msapifo.weyyak.z5.com/v1/en/contents/contentType?contentType= 
export CONTENT_TYPE_URL_PAGINATION=&pageNo=1&OrderBy=desc&RowCountPerPage=50&IsPaging=0 
export USER_LOG_URL=https://msapiqa-events.z5.com/event/log 
export S3_BUCKET=z5content-uat 
export S3_URLFORCONFIG=https://z5content-uat.s3.ap-south-1.amazonaws.com/configqa.json 
export CONFIG_KEY=configqa 
export REDIS_CACHE_URL=https://msapiqa-events.z5.com/cache 
export REDIS_CONTENT_KEY=GOAPIQA 
export DOTNET_URL=https://uat-api.weyyak.z5.com/v1/ar/oauth2/tokendata?access_token=
export PATH=$(go env GOPATH)/bin:$PATH
export JEAGER_URL=https://jaeger-tracer.weyyak.com/api/traces
export JEAGER_SERVICE_NAME=Frontend
# swag init -g main.go
go run *.go
