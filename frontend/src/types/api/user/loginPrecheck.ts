export interface PayloadProps {
	ssoEnabled: boolean;
	samlEnabled: boolean;
	samlLoginUrl: string;
}

export interface Props {
	email?: string;
	path?: string;
}
