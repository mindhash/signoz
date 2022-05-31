import getVersion from 'api/user/getVersion';
import { useState } from 'react';
import { useQueries } from 'react-query';

// todo(amol): need to add EE flag fetch and memo
function useEEAvailable(): string | undefined {
	return 'Y';
}

export default useEEAvailable;
