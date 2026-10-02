import { useState } from "react";

// A timestamp taken once when the screen opens; enough for "waiting 55m" fallbacks (the API normally supplies the minutes).
export const useNow = () => useState(() => Date.now())[0];
