export SERVICE_PORT=2000 
export    DB_SERVER=34.18.48.27 
export    DB_SERVER_READER=34.18.48.27 
export    DB_PORT=5432 
export    DB_USER=appuser 
export    DB_PASSWORD=Yy017EUY5aSz 
export    DB_DATABASE=wk_user_management 
export DB_DATABASE=wk_user_management 
export CONTENT_DB_DATABASE=wyk_content 
export FRONTEND_DB_DATABASE=wyk_frontend_config
export TEMPLATE_URL=/templates/ 
export EMAILIMAGEBASEURL=https://s3.ap-south-1.amazonaws.com/mailtemp/ 
export EMAILHEADIMAGEFILENAME=logo.png 
export EMAILCONTENTIMAGEFILENAME=devices.png 
export REDIRECTION_URL=https://weyyak.com/ 
export ADMIN_MAIL=marathon007@mailnesia.com 
export DEFAULT_PAGE_SIZE=20 
export AWS_REGION=ap-south-1 
export ACCESS_SECRET=AKIAYOGUWMUMEEQD6CPW 
export REFRESH_SECRET=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  
export PASSWORDCHANGEURL=https://backoffice-qa.engro.in/password?
export BOPASSWORDCHANGEURL=https://backoffice.weyyak.com/reset-password? 
export ReCAPTCHA_SECRET_web=6Lfnd3cjAAAAAEXYwjB7vXxpD5EJqgRhGAR9snm7 
export ReCAPTCHA_SECRET_ios=6Ld3UtsiAAAAAKBDjrIBbsZfD3ujFQyT01XIpntd 
export ReCAPTCHA_SECRET_android=6Lc7Z90kAAAAAL1I-QNxCj9CgSKkJJGaVJG1SkR4 
export BASE_URL=https://weyyak.com
export EGYPTBASE_URL=https://qa-weyyak1.z5.com 
export SUBSCRIPTION_URL=https://zpapi-prod.weyyak.com/orders/ 
export USER_DELETE_URL=https://zpapi-prod.weyyak.com/payment/registration/delete?id= 
export SES_REGION=ap-south-1 
export SES_ID=AKIAYOGUWMUMK2O4DT6B 
export SES_SECRET=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 
export DOTNET_URL=https://api-backoffice-production.weyyak.com/oauth2/tokendata?access_token= 
export TWITTER_CONSUMER_KEY=9eZDfmSeYROq2unPSqEXbIKrH 
export TWITTER_CONSUMER_SECRET_KEY=WqPZJ3uloFI876gKosGb4zujv2cW8TD4X5jPeWgU7pAd9Mxbmq 
export SESSION_SECRET="70Ie7PiMuS8JUIl1n-CcEP07Les5Y7Nk-eBc8x0jaHLz8ilfow" 
export JEAGER_URL=http://localhost:14268/api/traces
export JEAGER_SERVICE_NAME=User

# export PATH=$(go env GOPATH)/bin:$PATH
# swag init -g main.go
go run *.go


# go get -u github.com/swaggo/swag/cmd/swag