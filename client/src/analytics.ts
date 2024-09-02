import ReactGA from 'react-ga';

const TRACKING_ID = "G-6XZ35JY46Z"; 
ReactGA.initialize(TRACKING_ID);

export const trackPageView = (page: string) => {
  ReactGA.pageview(page);
};

export const trackEvent = (category: string, action: string, label: string = '') => {
  ReactGA.event({
    category: category,
    action: action,
    label: label
  });
};