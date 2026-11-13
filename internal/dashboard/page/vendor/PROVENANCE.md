# Vendored renderer

The graph card draws with deck.gl, vendored here and pinned to one version
(spec/backlog/dashboard-graph.md, decision 3). Nothing is fetched at runtime.

| | |
|---|---|
| Package | npm `deck.gl@9.3.12`, file `dist.min.js` (the standalone bundle, global `deck`) |
| Tarball | https://registry.npmjs.org/deck.gl/-/deck.gl-9.3.12.tgz |
| Tarball integrity (npm) | `sha512-+yP0dlxHod209CJ66NHhhfBn4OCpMdZvLUVnxaQmVXesW9pzeyKwdA7sjQ5h64K48R1/vsMOX2AwNWkOKiYdPw==` |
| Same file on unpkg | https://unpkg.com/deck.gl@9.3.12/dist.min.js |
| sha256 of `dist.min.js` | `9a58a99e0590185cd4e12acbfece94cd77ad433688a5b27f184731704d03f108` |
| Licence | MIT, `package/LICENSE` copied to `LICENSE-deckgl` |

The file is stored as `deckgl-9.3.12.min.js.gz` (gzip -9 -n), which the
dashboard serves with `Content-Encoding: gzip` to a client that accepts it and
decompresses for any other. `TestVendor_DeckGLIsTheUpstreamFileWithItsLicence`
pins the sha256 above against the decompressed bytes.

## Fetching it again, or the next version

```sh
V=9.3.12
curl -sSLO "https://registry.npmjs.org/deck.gl/-/deck.gl-$V.tgz"
npm view "deck.gl@$V" dist.integrity          # must equal:
printf 'sha512-%s\n' "$(openssl dgst -sha512 -binary "deck.gl-$V.tgz" | base64 | tr -d '\n')"
tar -xzf "deck.gl-$V.tgz" package/dist.min.js package/LICENSE
shasum -a 256 package/dist.min.js              # record it here and in deckSHA256
gzip -9 -n -c package/dist.min.js > "internal/dashboard/page/vendor/deckgl-$V.min.js.gz"
cp package/LICENSE internal/dashboard/page/vendor/LICENSE-deckgl
```

An upgrade renames the file, so `graph.js`'s `DECK_SRC`, the tests' file name
and `deckSHA256` change with it.
