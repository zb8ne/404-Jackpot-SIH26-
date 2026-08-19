// The one theme this demo ships: a government registry floor.
//
// The map, tilesets and monitor gids are lifted verbatim from the source
// project's office theme — those numbers are load-bearing (firstgid values index
// into the atlases, and the monitor gids identify which desk tiles light up).
// Everything the source carried for its own purposes — cafeteria seats, the
// coffee economy, idle errand anchors, a TV-show cast — is deliberately absent.

import type { TiledMap } from './engine/TiledMapRenderer';
import type { MonitorConfig } from './engine/themeRegistry';

import officeTilesetUrl from './assets/tilesets/office-tileset.png?url';
import a5FloorsWallsUrl from './assets/tilesets/a5-office-floors-walls.png?url';
import interiorsUrl from './assets/tilesets/interiors.png?url';
// .tmj is Tiled JSON, imported as raw text and parsed below.
import officeMapRaw from './assets/maps/office.tmj?raw';

interface TilesetEntry {
  url: string;
  /** True when the map carries this atlas's metadata inline; keep the map's copy. */
  embedded?: boolean;
  firstgid?: number;
  image?: string;
  imagewidth?: number;
  imageheight?: number;
  tilewidth?: number;
  tileheight?: number;
  columns?: number;
  tilecount?: number;
}

/** Who stands where. Spawn-point names come from the map's `spawn-points`
 *  object layer; we reuse the existing desks rather than authoring a new map. */
export type ActorId = 'birth' | 'transport' | 'education' | 'citizen' | 'verifier';

export interface ActorSeat {
  id: ActorId;
  /** Shown on the nameplate above the sprite. */
  label: string;
  /** A spawn-point name that exists in office.tmj. */
  seatName: string;
}

export const ACTOR_SEATS: ActorSeat[] = [
  { id: 'birth', label: 'Birth Dept', seatName: 'desk-team-lead' },
  { id: 'transport', label: 'Transport Dept', seatName: 'desk-backend-engineer' },
  { id: 'education', label: 'Education Dept', seatName: 'desk-product-manager' },
  { id: 'verifier', label: 'Verifier', seatName: 'desk-ceo' },
  { id: 'citizen', label: 'Citizen', seatName: 'desk-data-engineer' },
];

const TILESETS: TilesetEntry[] = [
  // office-tileset.png is embedded in the map at firstgid 1; keep the map's copy.
  { url: officeTilesetUrl, embedded: true },
  {
    url: a5FloorsWallsUrl,
    firstgid: 513,
    image: 'a5',
    imagewidth: 256,
    imageheight: 512,
    tilewidth: 16,
    tileheight: 16,
    columns: 16,
    tilecount: 512,
  },
  {
    url: interiorsUrl,
    firstgid: 1025,
    image: 'interiors',
    imagewidth: 256,
    imageheight: 1424,
    tilewidth: 16,
    tileheight: 16,
    columns: 16,
    tilecount: 1424,
  },
];

export const MONITOR: MonitorConfig = {
  offTopLeftGid: 365,
  onGids: [
    [367, 0, 0],
    [368, 1, 0],
    [383, 0, 1],
    [384, 1, 1],
  ],
};

/** Clear colour behind the floor. */
export const BACKGROUND = 0x14161c;

/** The ordered tileset image URLs; texture[i] must line up with tilesets[i]. */
export function tilesetUrls(): string[] {
  return TILESETS.map((t) => t.url);
}

/** Parse the Tiled JSON and patch in each atlas's metadata. Embedded atlases
 *  keep the map's own inline copy; the rest are replaced by the entries above. */
export function resolveMap(): TiledMap {
  const map = JSON.parse(officeMapRaw) as TiledMap;
  return {
    ...map,
    tilesets: TILESETS.map((entry, i) => {
      if (entry.embedded) return map.tilesets[i];
      const { url: _url, embedded: _embedded, ...meta } = entry;
      return meta as TiledMap['tilesets'][number];
    }),
  };
}
