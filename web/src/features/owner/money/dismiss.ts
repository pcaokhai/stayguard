import { messageFor } from "../../problem/problem";

export const NOTE_MAX = 500;

// Why a dismissal note cannot be sent yet; null when it can. The API requires 1–500 characters.
export const noteProblem = (note: string): "required" | "tooLong" | null => {
  const n = note.trim();
  return n.length === 0 ? "required" : n.length > NOTE_MAX ? "tooLong" : null;
};

// Text for a refused dismissal, by problem code (never the HTTP status).
export const dismissErrorMessage = (e: unknown) =>
  messageFor(e, { EVENT_NOT_DISMISSABLE: "money.notDismissable" });
