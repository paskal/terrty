# resume.json generation, html and pdf
resume:
	docker run -it --rm --name build-resume \
		--mount type=bind,source=$(PWD)/blog/public/cv,target=/data/ \
		paskal/jsonresume \
		export --theme kendall verhoturov.html
	docker run --rm --name build-pdf \
		--mount type=bind,source=$(PWD)/blog/public/cv,target=/tmp/html-to-pdf \
		--privileged pink33n/html-to-pdf \
		--url https://terrty.net/cv/verhoturov.html \
		--pdf verhoturov.pdf

# blog generation
blog:
	cd blog
	hugo

# build image with latest jsonresume-theme-kendall version
build-image-resumejson:
	docker build -t paskal/jsonresume cv
