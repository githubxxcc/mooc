package edu.coursera.concurrent;

import edu.rice.pcdp.Actor;
import static edu.rice.pcdp.PCDP.finish;
/**
 * An actor-based implementation of the Sieve of Eratosthenes.
 *
 * TODO Fill in the empty SieveActorActor actor class below and use it from
 * countPrimes to determin the number of primes <= limit.
 */
public final class SieveActor extends Sieve {
    /**
     * {@inheritDoc}
     *
     * TODO Use the SieveActorActor class to calculate the number of primes <=
     * limit in parallel. You might consider how you can model the Sieve of
     * Eratosthenes as a pipeline of actors, each corresponding to a single
     * prime number.
     */
	
	private final int MAX_LOCAL_PRIMES = 10;
	
    @Override
    public int countPrimes(final int limit) {
    	if(limit <= 2) return limit/2;
    	int cumCount = 0;
    	
    	final SieveActorActor actor = new SieveActorActor();
    	
    	finish(()->{
    		for(int i = 3; i<=limit; i+=2){
        		actor.send(i);
        	}
    	});
    	
    	SieveActorActor curActor = actor;
    	
    	//Count the prime
    	while(curActor != null) {
    		cumCount++;
    		curActor = curActor.next;
    	}
    		
    	return cumCount;
    }

    /**
     * An actor class that helps implement the Sieve of Eratosthenes in
     * parallel.
     */
    public static final class SieveActorActor extends Actor {
        /**
         * Process a single message sent to this actor.
         *
         * TODO complete this method.
         *
         * @param msg Received message
         */
    	private SieveActorActor next;
    	private Integer localNum;
    	
        @Override
        public void process(final Object msg) {
        	final int num = (Integer) msg;
        	if(next == null){
        		localNum = num;
        		next = new SieveActorActor();
        	} else {
        		if(num % localNum != 0)
        			next.send(num);
        	}
        }
    }
}
