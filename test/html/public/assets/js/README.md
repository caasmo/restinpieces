esbuild test/html/public/assets/js/restinpieces.js --outfile=restinpieces.min.js \
 --bundle \
 --minify \
 --sourcemap=external \
 --drop:console \
 --format=esm \
 --target=es2017 \
 --platform=browser
