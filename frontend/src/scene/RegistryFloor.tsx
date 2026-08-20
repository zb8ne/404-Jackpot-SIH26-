// The registry floor: a Pixi scene showing departments, a citizen and a verifier
// as characters at desks. It renders the map and the cast; it deliberately knows
// nothing about the registry's API — events are pushed in through the imperative
// handle so the same scene serves both the guided and the live demo.

import { useEffect, useRef, useState } from 'react';
import { Application, Container, Texture } from 'pixi.js';
// Pixi builds shaders with new Function(); this shim keeps it working wherever a
// Content-Security-Policy forbids that.
import 'pixi.js/unsafe-eval';

import { Camera } from './engine/Camera';
import { Character } from './engine/Character';
import { SpriteAdapter } from './engine/SpriteAdapter';
import { TiledMapRenderer } from './engine/TiledMapRenderer';
import { installContextLossRecovery } from './engine/glRecovery';
import { ACTOR_SEATS, BACKGROUND, resolveMap, tilesetUrls, type ActorId } from './registryTheme';
import { createChoreographer } from './choreography';
import type { SceneApi } from './sceneApi';
import { colors } from './tokens';

import adamSheet from './assets/characters/Adam_walk.png?url';
import alexSheet from './assets/characters/Alex_walk.png?url';
import ameliaSheet from './assets/characters/Amelia_walk.png?url';
import bobSheet from './assets/characters/Bob_walk.png?url';

/** LimeZu walk sheets: 16x32 frames, walk cycle on row 1, six frames per direction. */
const SHEET_CONFIG = { frameWidth: 16, frameHeight: 32, walkRow: 1, framesPerDirection: 6 };

const SHEET_BY_ACTOR: Record<ActorId, string> = {
  birth: adamSheet,
  transport: alexSheet,
  education: ameliaSheet,
  citizen: bobSheet,
  // The verifier reuses a sheet — four sheets, five actors.
  verifier: adamSheet,
};

const GLOW_BY_ACTOR: Record<ActorId, number> = {
  birth: colors.accent.sky,
  transport: colors.accent.mint,
  education: colors.accent.lilac,
  citizen: colors.accent.lemon,
  verifier: colors.accent.coral,
};

function loadTexture(url: string): Promise<Texture> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => {
      const tex = Texture.from(img);
      // Pixel art: never smooth it.
      tex.source.scaleMode = 'nearest';
      resolve(tex);
    };
    img.onerror = () => reject(new Error('failed to load ' + url.slice(0, 60)));
    img.src = url;
  });
}

function safeDestroy(app: Application) {
  try {
    app.ticker?.stop();
  } catch {
    /* already gone */
  }
  try {
    app.destroy(true, { children: true });
  } catch {
    /* already gone */
  }
}

interface RegistryFloorProps {
  /** Handed the imperative handle once the floor is live. */
  onReady?: (api: SceneApi) => void;
  /** Caption for whatever is playing; '' when the floor is idle. */
  onCaption?: (caption: string) => void;
}

export function RegistryFloor({ onReady, onCaption }: RegistryFloorProps = {}) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const appRef = useRef<Application | null>(null);
  const mountIdRef = useRef(0);
  // Bumped when the GPU evicts our WebGL context, so the effect below rebuilds
  // the whole scene through the normal mount path rather than a second routine.
  const [glGeneration, setGlGeneration] = useState(0);
  const [error, setError] = useState('');
  // Held in refs so a parent re-render never tears the Pixi scene down.
  const onReadyRef = useRef(onReady);
  const onCaptionRef = useRef(onCaption);
  onReadyRef.current = onReady;
  onCaptionRef.current = onCaption;

  // A hidden tab still runs the ticker and draws every frame into pixels nobody
  // sees. Stop the ticker instead of unmounting: textures and the scene graph
  // stay alive, so coming back is instant.
  const [docHidden, setDocHidden] = useState(() => document.hidden);
  useEffect(() => {
    const onVis = () => setDocHidden(document.hidden);
    document.addEventListener('visibilitychange', onVis);
    return () => document.removeEventListener('visibilitychange', onVis);
  }, []);

  useEffect(() => {
    const ticker = appRef.current?.ticker;
    if (!ticker) return;
    if (docHidden) ticker.stop();
    else ticker.start();
  }, [docHidden]);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    while (host.firstChild) host.removeChild(host.firstChild);

    const mountId = ++mountIdRef.current;
    const app = new Application();
    appRef.current = app;
    const characters = new Map<ActorId, Character>();

    const init = async () => {
      await app.init({
        background: BACKGROUND,
        antialias: false,
        roundPixels: true,
        // Render at real device pixel density (floored at 2) or the OS upscales
        // the canvas on HiDPI displays and blurs the pixel art.
        resolution: Math.max(window.devicePixelRatio || 1, 2),
        autoDensity: true,
        width: host.clientWidth || 800,
        height: host.clientHeight || 600,
      });
      // The scene may have been torn down while init awaited.
      if (mountIdRef.current !== mountId) return safeDestroy(app);

      while (host.firstChild) host.removeChild(host.firstChild);
      host.appendChild(app.canvas);

      installContextLossRecovery(app.canvas, {
        onRebuild: () => {
          if (mountIdRef.current === mountId) setGlGeneration((n) => n + 1);
        },
        onGiveUp: () => {
          if (mountIdRef.current === mountId) setError('The floor lost its GPU context. Reload to bring it back.');
        },
      });

      // Tilesets load in theme order: texture[i] lines up with map tilesets[i].
      const textures = await Promise.all(tilesetUrls().map(loadTexture));
      if (mountIdRef.current !== mountId) return safeDestroy(app);

      const world = new Container();
      app.stage.addChild(world);

      const mapRenderer = new TiledMapRenderer(resolveMap(), textures);
      world.addChild(mapRenderer.getContainer());
      const charLayer = mapRenderer.getCharacterContainer();

      if (import.meta.env.DEV) {
        const tileSprites = mapRenderer
          .getContainer()
          .children.reduce((n, c) => n + ((c as Container).children?.length ?? 0), 0);
        console.log(
          `[RegistryFloor] map ${mapRenderer.width}x${mapRenderer.height} tiles, ${tileSprites} sprites`,
        );
      }

      const camera = new Camera(world);
      camera.setMapSize(mapRenderer.width * mapRenderer.tileSize, mapRenderer.height * mapRenderer.tileSize);
      camera.setViewSize(app.screen.width, app.screen.height);
      camera.fitToScreen();
      // Land on the framed view immediately, so the first painted frame is right.
      camera.snap();

      const entrance = mapRenderer.getSpawnPoint('entrance');

      for (const seat of ACTOR_SEATS) {
        const sheet = await loadTexture(SHEET_BY_ACTOR[seat.id]);
        if (mountIdRef.current !== mountId) return safeDestroy(app);

        const seatTile = mapRenderer.getSpawnPoint(seat.seatName) ?? entrance ?? { x: 2, y: 2 };
        const character = new Character({
          agentId: seat.id,
          mapRenderer,
          frames: SpriteAdapter.extractFrames(sheet, SHEET_CONFIG),
          seatTile,
          spawnTile: entrance ?? seatTile,
          glowColor: GLOW_BY_ACTOR[seat.id],
        });
        character.show(charLayer);
        // Walk in from the door, then take the desk.
        character.walkToAndThen(seatTile, () => character.sitAtDesk(false));
        characters.set(seat.id, character);
      }

      const onResize = () => {
        if (mountIdRef.current !== mountId) return;
        app.renderer.resize(host.clientWidth || 800, host.clientHeight || 600);
        camera.setViewSize(app.screen.width, app.screen.height);
        camera.fitToScreen();
        if (!app.ticker.started) {
          camera.snap();
          app.render();
        }
      };
      window.addEventListener('resize', onResize);
      (app as unknown as { __onResize: () => void }).__onResize = onResize;

      const choreographer = createChoreographer({
        characters,
        layer: charLayer,
        camera,
        onCaption: (caption) => onCaptionRef.current?.(caption),
      });

      app.ticker.add((ticker) => {
        const dt = ticker.deltaMS / 1000;
        camera.update(dt);
        for (const character of characters.values()) character.update(dt);
        choreographer.update(dt);
      });

      onReadyRef.current?.({
        play: (event) => choreographer.play(event),
        reset: () => choreographer.reset(),
        pending: () => choreographer.pending(),
      });

      // Paint one frame unconditionally. Without this a floor built while the tab
      // is in the background (or on a machine where the page reports hidden)
      // stays black until something restarts the ticker.
      app.render();
      if (document.hidden) app.ticker.stop();

      // Dev-only handle. Invaluable when the floor comes up blank: a backgrounded
      // tab gets no requestAnimationFrame, so nothing animates and the scene looks
      // broken when it is merely paused. Pump `characters` by hand to check.
      if (import.meta.env.DEV) {
        (window as unknown as { __floor: unknown }).__floor = { app, world, camera, mapRenderer, characters, choreographer };
      }
    };

    void init().catch((err: unknown) => {
      if (mountIdRef.current !== mountId) return;
      setError(err instanceof Error ? err.message : String(err));
    });

    return () => {
      mountIdRef.current++;
      const stored = (app as unknown as { __onResize?: () => void }).__onResize;
      if (stored) window.removeEventListener('resize', stored);
      for (const character of characters.values()) character.destroy();
      characters.clear();
      safeDestroy(app);
    };
  }, [glGeneration]);

  return (
    <div className="relative h-[clamp(440px,72vh,820px)] min-h-0 w-full overflow-hidden rounded-2xl border border-slate-800 bg-slate-950">
      <div ref={hostRef} className="h-full w-full" />
      {error && (
        <div className="absolute inset-0 flex items-center justify-center p-6 text-center text-red-300">
          {error}
        </div>
      )}
    </div>
  );
}
