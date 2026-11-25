import { randomUUID } from "node:crypto";
import fs from "node:fs";
import path from "node:path";

const DATA_DIR = process.env.DAG_DATA_DIR || path.join(process.cwd(), ".data");
const DATA_FILE = path.join(DATA_DIR, "store.json");

function empty() {
  return {
    sessionToken: null,
    southChannels: [],
    points: [],
    thingModels: [],
    northTargets: [],
    events: [],
    northResults: {},
  };
}

export function createStore() {
  let data = empty();
  try {
    if (fs.existsSync(DATA_FILE)) {
      data = { ...empty(), ...JSON.parse(fs.readFileSync(DATA_FILE, "utf8")) };
    }
  } catch {
    data = empty();
  }

  function persist() {
    fs.mkdirSync(DATA_DIR, { recursive: true });
    fs.writeFileSync(DATA_FILE, JSON.stringify(data, null, 2));
  }

  function pushEvent(evt) {
    data.events.unshift({
      id: randomUUID(),
      time: new Date().toISOString(),
      ...evt,
    });
    data.events = data.events.slice(0, 200);
    persist();
  }

  return {
    get raw() {
      return data;
    },
    persist,
    pushEvent,
    reset() {
      data = empty();
      persist();
    },
    id: () => randomUUID(),
  };
}
