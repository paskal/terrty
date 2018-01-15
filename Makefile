# DNS relay to make VPN work without any additional settings
dns-container:
	docker run -d --restart=always -p 53\:53/tcp \
		-p 53\:53/udp \
		--cap-add=NET_ADMIN \
		andyshinn/dnsmasq\:latest

# VPN server
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

# point.im telegram bot hosting
pointim_bot-container:
	. ../credentials.conf && \
	docker run -d --restart=always \
		--mount type=bind,source=/root/cache.bin,target=/usr/pointim_bot/cache.bin \
		-e LOGIN="$${POINT_BOT_LOGIN}" \
		-e PASSWORD="$${POINT_BOT_PASSWORD}" \
		paskal/pointim_bot

# resume.json generation, html and pdf
build-resume:
	docker run -it --rm --name build-resume \
		--mount type=bind,source=$(PWD)/cv,target=/data/ \
		paskal/jsonresume \
		export --theme kendall verhoturov.html
	mkdir -p blog/public/cv/
	mv cv/verhoturov.html blog/public/cv/
	# https://github.com/LinuxBozo/jsonresume-theme-kendall/issues/15 fix
	sed -i 's/.job-details,/#.job-details,/g' ./blog/public/cv/verhoturov.html
	docker run --rm --name build-pdf \
		--mount type=bind,source=$(PWD)/blog/public/cv,target=/tmp/html-to-pdf \
		--privileged pink33n/html-to-pdf \
		--url https://terrty.net/cv/verhoturov.html \
		--pdf verhoturov.pdf

# blog generation
build-blog:
	cd blog
	hugo

# build image with latest jsonresume-theme-kendall version
build-image-resumejson:
	docker build -t paskal/jsonresume cv

# build image with customized octopress theme
build-image-blog:
	docker build -t paskal/octopress blog

# build image for https://github.com/deemess/pointim_bot
build-image-pointim_bot:
	docker build -t paskal/pointim_bot pointim_bot
