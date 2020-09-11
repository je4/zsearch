Create Amp Packager Certificate

    $ openssl ecparam -out mediathek-amp-packager.key -name prime256v1 -genkey
    $ openssl req -new -key mediathek-amp-packager.key -nodes -out mediathek-amp-packager.csr -subj "/C=CH/ST=Basel_Stadt/L=Basel/O=FHNW/CN=mediathek.hgk.fhnw.ch"
    $ openssl x509 -req -days 90 -in mediathek-amp-packager.csr -signkey mediathek-amp-packager.key -out mediathek-amp-packager.pem -extfile <(echo -e "1.3.6.1.4.1.11129.2.1.22 = ASN1:NULL\nsubjectAltName=DNS:mediathek.hgk.fhnw.ch")
    Signature ok
    subject=C = CH, ST = Basel_Stadt, L = Basel, O = FHNW, CN = mediathek.hgk.fhnw.ch
    Getting Private key
    $