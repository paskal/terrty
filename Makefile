dns-container:
	docker run -d --restart=always -p 53\:53/tcp \
		-p 53\:53/udp \
		--cap-add=NET_ADMIN \
		andyshinn/dnsmasq\:latest

vpn-container:
	. ../credentials.conf && \
	docker run -d --restart=always \
		--cap-add NET_ADMIN \
		-p 500\:500/udp -p 4500\:4500/udp \
		-p 1701\:1701/tcp -p 1194\:1194/udp \
		-p 5555\:5555/tcp -p 992\:992/tcp \
		-e USERS="$${VPN_LOGIN}"\:"$${VPN_PASSWORD}" \
		-e PSK=$${VPN_SHARED_KEY} \
		siomiz/softethervpn

nginx-container:
	docker run -d --restart=always \
		-p 80\:80 -p 443\:443 \
		--mount type=bind,source=/root/terrty/blog/public,target=/usr/share/nginx/html,readonly \
		--mount type=bind,source=/root/terrty/nginx,target=/etc/nginx/conf.d,readonly \
		--mount type=bind,source=/etc/letsencrypt,target=/etc/letsencrypt,readonly \
		nginx\:alpine

certbot-container:
	docker run --rm \
		--mount type=bind,source=/etc/letsencrypt,target=/etc/letsencrypt \
		certbot/certbot \
		certonly -d terrty.net --standalone -m paskal.07@gmail.com --agree-tos

pointim_bot-container:
	. ../credentials.conf && \
	docker run -d --restart=always \
		--mount type=bind,source=/root/cache.bin,target=/usr/src/pointim_bot-master/target/cache.bin \
		-e LOGIN="$${POINT_BOT_LOGIN}" \
		-e PASSWORD="$${POINT_BOT_PASSWORD}" \
		paskal/pointim_bot

build-resume:
	docker run -it --rm --name build-resume \
		--mount type=bind,source=$(PWD)/cv,target=/data/ \
		paskal/jsonresume \
		export --theme kendall verhoturov.html
	mkdir -p blog/public/cv/
	rm -f blog/public/cv/* || true
	mv cv/verhoturov.html blog/public/cv/
	xvfb-run wkhtmltopdf blog/public/cv/verhoturov.html blog/public/cv/verhoturov.pdf

build-blog:
	docker run -it --rm \
		--mount type=bind,source=$(PWD)/blog/source/,target=/data/source/ \
		--mount type=bind,source=$(PWD)/blog/public/,target=/data/public/ \
		paskal/octopress \
		generate

build-image-resumejson:
	docker build -t paskal/jsonresume cv

build-image-blog:
	docker build -t paskal/octopress blog

build-image-pointim_bot:
	docker build -t paskal/pointim_bot pointim_bot
