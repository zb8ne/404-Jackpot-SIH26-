// Turns registry events into things that happen on the floor.
//
// Events play strictly one at a time. Five characters all moving at once reads
// as noise; a queue makes the story legible, and it means a burst of activity
// becomes a sequence rather than a scramble.
//
// Every wait is measured in simulated time (accumulated from the ticker) rather
// than wall-clock. So a paused floor pauses the story too, and the whole thing
// can be stepped by hand — which is the only way to inspect it in a backgrounded
// tab, where the browser grants no animation frames.

import type { Container } from 'pixi.js';

import type { Camera } from './engine/Camera';
import type { Character } from './engine/Character';
import { MessageEnvelope, type MessageAct } from './engine/MessageEnvelope';
import type { ActorId } from './registryTheme';
import { VERDICT_COPY, describe, type SceneEvent, type Verdict } from './sceneApi';

interface Deps {
  characters: Map<ActorId, Character>;
  layer: Container;
  camera: Camera;
  /** Called with a caption whenever the playing event changes; '' when idle. */
  onCaption?: (caption: string) => void;
}

/** How a verdict reads on the floor. */
const VERDICT_ACT: Record<Verdict, MessageAct> = {
  VALID: 'agree',
  SUPERSEDED: 'inform',
  REVOKED: 'refuse',
  TAMPERED: 'refuse',
  NOT_ISSUED: 'refuse',
};

export function createChoreographer(deps: Deps) {
  const { characters, layer, camera } = deps;
  const caption = deps.onCaption ?? (() => {});

  /** Envelopes in flight, each with the resolver waiting on its arrival. */
  const flights: Array<{ envelope: MessageEnvelope; resolve: () => void }> = [];
  /** Sleepers waiting for simulated time to pass. */
  const timers: Array<{ remaining: number; resolve: () => void }> = [];

  const queue: SceneEvent[] = [];
  let running = false;

  const actor = (id: ActorId) => characters.get(id);

  function wait(seconds: number): Promise<void> {
    return new Promise((resolve) => timers.push({ remaining: seconds, resolve }));
  }

  /** Fly an envelope between two characters and resolve when it lands. */
  function fly(fromId: ActorId, toId: ActorId, act: MessageAct): Promise<void> {
    const from = actor(fromId);
    const to = actor(toId);
    if (!from || !to) return Promise.resolve();

    const envelope = new MessageEnvelope(from.getPixelPosition(), to.getPixelPosition(), act, false);
    layer.addChild(envelope.container);
    return new Promise((resolve) => flights.push({ envelope, resolve }));
  }

  /** Pan gently toward whoever is acting, so the eye follows the story. */
  function look(at: ActorId) {
    const who = actor(at);
    if (who) camera.nudgeToward(who.getPixelPosition().x, who.getPixelPosition().y);
  }

  function say(id: ActorId, text: string) {
    actor(id)?.showThought(text);
  }

  function clearBubbles() {
    for (const character of characters.values()) {
      character.hideThought();
      character.setStatusGlyph('none');
    }
  }

  async function perform(event: SceneEvent): Promise<void> {
    switch (event.kind) {
      case 'ISSUE': {
        look(event.dept);
        say(event.dept, `issuing ${event.docId}`);
        await wait(0.9);

        if (!event.ok) {
          // The overreach: the document leaves the desk and comes straight back.
          await fly(event.dept, 'citizen', 'refuse');
          await fly('citizen', event.dept, 'refuse');
          actor(event.dept)?.setStatusGlyph('blocked');
          say(event.dept, event.reason ?? 'not my document type');
          await wait(1.6);
          return;
        }

        await fly(event.dept, 'citizen', 'inform');
        actor(event.dept)?.setStatusGlyph('success');
        actor('citizen')?.cheer();
        say('citizen', `holds ${event.docId}`);
        await wait(1.2);
        return;
      }

      case 'VERIFY': {
        look('verifier');
        say('verifier', `checking ${event.docId}`);
        await wait(0.7);
        await fly('verifier', 'citizen', 'query');
        await fly('citizen', 'verifier', VERDICT_ACT[event.verdict]);
        actor('verifier')?.setStatusGlyph(event.verdict === 'VALID' ? 'success' : 'blocked');
        say('verifier', VERDICT_COPY[event.verdict]);
        await wait(1.8);
        return;
      }

      case 'SUPERSEDE': {
        look(event.dept);
        say(event.dept, `correcting ${event.docId}`);
        await wait(0.9);
        await fly(event.dept, 'citizen', 'inform');
        say('citizen', `${event.newDocId} is current`);
        actor(event.dept)?.setStatusGlyph('success');
        await wait(1.5);
        return;
      }

      case 'REVOKE': {
        look(event.dept);
        say(event.dept, `revoking ${event.docId}`);
        await wait(0.8);
        await fly(event.dept, 'citizen', 'refuse');
        actor('citizen')?.setStatusGlyph('blocked');
        say('citizen', `${event.docId} withdrawn`);
        await wait(1.5);
        return;
      }

      case 'CONSENT_REQUEST': {
        look('citizen');
        await fly('verifier', 'citizen', 'request');
        say('citizen', 'may they check it?');
        await wait(1.4);
        return;
      }

      case 'CONSENT_APPROVED':
      case 'CONSENT_DENIED': {
        const approved = event.kind === 'CONSENT_APPROVED';
        look('verifier');
        await fly('citizen', 'verifier', approved ? 'agree' : 'refuse');
        actor('verifier')?.setStatusGlyph(approved ? 'success' : 'blocked');
        say('verifier', approved ? 'consent given' : 'consent refused');
        await wait(1.4);
        return;
      }
    }
  }

  async function drain(): Promise<void> {
    if (running) return;
    running = true;
    while (queue.length > 0) {
      const event = queue.shift();
      if (!event) break;
      caption(describe(event));
      try {
        await perform(event);
      } catch {
        // One bad animation must not wedge the queue.
      }
      clearBubbles();
    }
    running = false;
    caption('');
  }

  return {
    play(event: SceneEvent) {
      queue.push(event);
      void drain();
    },

    reset() {
      queue.length = 0;
      for (const { envelope, resolve } of flights.splice(0)) {
        envelope.destroy();
        resolve();
      }
      for (const timer of timers.splice(0)) timer.resolve();
      clearBubbles();
      for (const character of characters.values()) character.sitAtDesk(false);
      camera.fitToScreen();
      caption('');
    },

    pending(): number {
      return queue.length + (running ? 1 : 0);
    },

    /** Drive flights and timers. Call once per frame from the ticker. */
    update(dt: number) {
      for (let i = flights.length - 1; i >= 0; i--) {
        const flight = flights[i];
        if (flight.envelope.update(dt)) {
          flight.envelope.destroy();
          flights.splice(i, 1);
          flight.resolve();
        }
      }
      for (let i = timers.length - 1; i >= 0; i--) {
        const timer = timers[i];
        timer.remaining -= dt;
        if (timer.remaining <= 0) {
          timers.splice(i, 1);
          timer.resolve();
        }
      }
    },
  };
}

export type Choreographer = ReturnType<typeof createChoreographer>;
