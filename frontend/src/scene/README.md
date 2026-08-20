# The registry floor

A Pixi.js tile scene that animates what the registry is doing: departments at
desks, documents as envelopes, verdicts in thought bubbles.

`engine/` is vendored from the `demo-visuals` project (an Electron app) and is
deliberately kept close to its original form so it can be re-synced. Two edits
were needed:

- `TiledMapRenderer.ts` — constructor parameter properties rewritten as explicit
  fields, because this project builds with `erasableSyntaxOnly`.
- `Camera.ts` — added `snap()`, which lands on the framed view immediately
  instead of lerping, so the first painted frame is correct.

Everything specific to the source project — its agent harness, cafeteria banter,
coffee economy, idle errands and TV-show cast — was left behind.

## Art

Tilesets and character sheets are LimeZu's free assets: **non-commercial use
only**, see `assets/tilesets/LIMEZUASSETS-LICENSE.txt`. Fine for a hackathon
demo; they must be replaced before any commercial use.

## Debugging a blank floor

A backgrounded browser tab gets no `requestAnimationFrame`, so the ticker stops
and nothing animates — the scene looks broken when it is merely paused. In dev,
`window.__floor` exposes `{ app, world, camera, mapRenderer, characters }`;
pump it by hand to confirm the scene is alive:

```js
const f = window.__floor
for (let i = 0; i < 720; i++) { f.camera.update(1/60); for (const c of f.characters.values()) c.update(1/60) }
f.app.render()
```
