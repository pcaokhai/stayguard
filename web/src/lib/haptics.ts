// One short buzz when a payment lands; skipped until the user has touched the page (browsers log an error otherwise).
export const buzz = () => {
  if (navigator.userActivation?.hasBeenActive) navigator.vibrate?.(15);
};
