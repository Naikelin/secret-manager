# SSL Certificate Setup

This directory should contain your SSL certificates for HTTPS.

## Required Files

- `cert.pem` - SSL certificate (public key)
- `key.pem` - SSL private key
- `chain.pem` - Certificate chain (optional, but recommended)

## Option 1: Let's Encrypt (Recommended)

Use Certbot to obtain free SSL certificates:

```bash
# Install certbot
sudo apt-get update
sudo apt-get install certbot

# Obtain certificate (standalone mode)
sudo certbot certonly --standalone -d your-domain.com

# Copy certificates to this directory
sudo cp /etc/letsencrypt/live/your-domain.com/fullchain.pem ./cert.pem
sudo cp /etc/letsencrypt/live/your-domain.com/privkey.pem ./key.pem
```

### Auto-Renewal

Add to crontab for automatic renewal:

```bash
0 0 1 * * certbot renew --quiet && docker-compose -f docker-compose.prod.yml restart nginx
```

## Option 2: Self-Signed Certificate (Development/Testing)

Generate a self-signed certificate:

```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout key.pem \
  -out cert.pem \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=your-domain.com"
```

**WARNING**: Self-signed certificates will show browser warnings. Use only for testing.

## Option 3: Commercial Certificate

If you purchased a certificate from a CA:

1. Copy the certificate file to `cert.pem`
2. Copy the private key to `key.pem`
3. Copy the CA bundle/chain to `chain.pem`

## File Permissions

Ensure proper permissions:

```bash
chmod 644 cert.pem chain.pem
chmod 600 key.pem
```

## Enable HTTPS

1. Place certificates in this directory
2. Copy `nginx/conf.d/ssl.conf.example` to `nginx/conf.d/ssl.conf`
3. Edit `nginx/conf.d/ssl.conf` and replace `your-domain.com` with your actual domain
4. Uncomment HTTPS redirect in `nginx/conf.d/default.conf`
5. Restart nginx: `docker-compose -f docker-compose.prod.yml restart nginx`

## Troubleshooting

### Test certificate validity

```bash
openssl x509 -in cert.pem -text -noout
```

### Test nginx configuration

```bash
docker-compose -f docker-compose.prod.yml exec nginx nginx -t
```

### Check certificate expiration

```bash
openssl x509 -in cert.pem -noout -enddate
```
